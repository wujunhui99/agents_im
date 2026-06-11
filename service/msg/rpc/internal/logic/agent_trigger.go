package logic

import (
	"context"
	"errors"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// NewAgentResponseSender 是 Kafka 模式下 AI 托管写回的实现（svc.BindAgentResponseSender
// 注入）：business 请求 → pb → 本进程 SendMessageLogic（即 publish msg.toTransfer.v1）。
// AI 回复与用户消息走同一条链路，seq 统一由 msgtransfer 的 Redis Malloc 分配。
func NewAgentResponseSender(svcCtx *svc.ServiceContext) svc.AgentSendFunc {
	return func(ctx context.Context, req business.SendMessageRequest) (business.SendMessageResponse, error) {
		resp, err := NewSendMessageLogic(ctx, svcCtx).SendMessage(&msg.SendMessageRequest{
			SenderId:              req.SenderID,
			ReceiverId:            req.ReceiverID,
			GroupId:               req.GroupID,
			ChatType:              req.ChatType,
			ClientMsgId:           req.ClientMsgID,
			ContentType:           req.ContentType,
			Content:               req.Content,
			MessageOrigin:         req.MessageOrigin,
			AgentAccountId:        req.AgentAccountID,
			TriggerServerMsgId:    req.TriggerServerMsgID,
			AgentRunId:            req.AgentRunID,
			AllowRecursiveTrigger: req.AllowRecursiveTrigger,
		})
		if err != nil {
			return business.SendMessageResponse{}, err
		}
		return business.SendMessageResponse{
			Message:      businessMessageFromPB(resp.GetMessage()),
			Deduplicated: resp.GetDeduplicated(),
		}, nil
	}
}

func businessMessageFromPB(m *msg.Message) business.Message {
	if m == nil {
		return business.Message{}
	}
	return business.Message{
		ServerMsgID:           m.GetServerMsgId(),
		ClientMsgID:           m.GetClientMsgId(),
		ConversationID:        m.GetConversationId(),
		Seq:                   m.GetSeq(),
		SenderID:              m.GetSenderId(),
		ReceiverID:            m.GetReceiverId(),
		GroupID:               m.GetGroupId(),
		ChatType:              m.GetChatType(),
		ContentType:           m.GetContentType(),
		Content:               m.GetContent(),
		MessageOrigin:         m.GetMessageOrigin(),
		AgentAccountID:        m.GetAgentAccountId(),
		TriggerServerMsgID:    m.GetTriggerServerMsgId(),
		AgentRunID:            m.GetAgentRunId(),
		AllowRecursiveTrigger: m.GetAllowRecursiveTrigger(),
		SendTime:              m.GetSendTime(),
		CreatedAt:             m.GetCreatedAt(),
	}
}

// RunAgentTriggerConsumer 消费 agent.trigger.v1（msgtransfer 对每条 storage 消息
// produce，03 §9 B1 偏差 (2)），把 seq 已就绪的消息回流给现有 AgentHook——hosting/
// recursion/dedup 判定全部保留在 runtime 内，语义对齐旧 fireMessageCreatedHook。
// 钩子错误只记日志、批次照常 commit（对齐旧路径"hook 失败不影响链路"）。
// 阻塞直到 ctx 结束；04-agent.md 落地后整体迁往 agent 域。
func RunAgentTriggerConsumer(ctx context.Context, svcCtx *svc.ServiceContext) error {
	if svcCtx.AgentHook == nil {
		return errors.New("agent trigger consumer requires AgentHook")
	}
	consumer, err := messaging.NewKafkaConsumer(svcCtx.KafkaBrokers, messaging.GroupAgentTrigger, []string{messaging.TopicAgentTrigger})
	if err != nil {
		return err
	}
	defer consumer.Close()
	return consumer.Run(ctx, func(ctx context.Context, records []*kgo.Record) error {
		for _, record := range records {
			event, err := messaging.UnmarshalMessageEvent(record.Value)
			if err != nil {
				logx.WithContext(ctx).Errorf("msg-rpc: drop malformed agent.trigger record offset=%d: %v", record.Offset, err)
				continue
			}
			if event.EventType != messaging.EventTypeMessageAccepted {
				continue
			}
			input := business.MessageCreatedHookInput{
				EventID:          "message.created:" + event.ServerMsgID,
				Message:          businessMessageFromEvent(event),
				Deduplicated:     false,
				RecipientUserIDs: append([]string(nil), event.Payload.ReceiverIDs...),
			}
			if err := svcCtx.AgentHook.OnMessageCreated(ctx, input); err != nil {
				logx.WithContext(ctx).Errorf("agent trigger hook failed server_msg_id=%q conversation_id=%q seq=%d: %v",
					event.ServerMsgID, event.ConversationID, event.Seq, err)
			}
		}
		return nil
	})
}

func businessMessageFromEvent(event messaging.MessageEvent) business.Message {
	sendTime := event.Payload.SendTime
	if sendTime == 0 {
		sendTime = event.CreatedAt
	}
	return business.Message{
		ServerMsgID:           event.ServerMsgID,
		ClientMsgID:           event.Payload.ClientMsgID,
		ConversationID:        event.ConversationID,
		Seq:                   event.Seq,
		SenderID:              event.SenderID,
		ReceiverID:            event.Payload.ReceiverID,
		GroupID:               event.Payload.GroupID,
		ChatType:              event.ChatType,
		ContentType:           event.Payload.ContentType,
		Content:               model.DecodeMessageContent(string(event.Payload.Content)),
		MessageOrigin:         event.Payload.MessageOrigin,
		AgentAccountID:        event.Payload.AgentAccountID,
		TriggerServerMsgID:    event.Payload.TriggerServerMsgID,
		AgentRunID:            event.Payload.AgentRunID,
		AllowRecursiveTrigger: event.Payload.AllowRecursiveTrigger,
		SendTime:              sendTime,
		CreatedAt:             event.CreatedAt,
	}
}
