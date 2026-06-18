package logic

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"time"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// sendDirectKafka 是 MSG_DIRECT_KAFKA 开启时的写路径（03 §4.2 / §9 B2）：
// 校验与 normalize 不变，但唯一写原语是 publish message.submitted 到
// msg.toTransfer.v1（MsgToMQ，对齐 OpenIM send.go）。不写 PG、不分配 seq、
// 不触发 in-process AI hook（触发经 msgtransfer → agent.trigger.v1 回流）。
// ACK 仅代表 Kafka acks=all 接受；seq 由客户端经自己的 message_received
// push 异步回填。Deduplicated 恒为 false（dedup 收敛在 msgtransfer）。
func (l *SendMessageLogic) sendDirectKafka(ns normalizedSend, payloadHash string) (*msg.SendMessageResponse, error) {
	// message_id 雪花 bigint（EPIC #527 §0）：中段最高位区分单/群（单=1，群=0），wire 仍十进制字符串（ADR #529）。
	serverMsgIDInt, err := l.svcCtx.MsgIDGen.Next(svc.MsgHintForChatType(ns.ChatType))
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("could not allocate message id"))
	}
	serverMsgID := strconv.FormatInt(serverMsgIDInt, 10)
	eventID, err := idgen.NewString()
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	contentJSON, err := model.EncodeMessageContent(ns.ContentType, ns.Content)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	now := time.Now().UTC()
	visible := model.VisibleUserIDs(ns.SenderID, ns.ReceiverID, ns.ChatType, ns.ParticipantUserIDs)
	trace := observability.TraceContextFromContext(l.ctx)

	event := messaging.MessageEvent{
		EventID:        eventID,
		EventType:      messaging.EventTypeMessageSubmitted,
		ConversationID: ns.ConversationID,
		ServerMsgID:    serverMsgID,
		SenderID:       ns.SenderID,
		ChatType:       ns.ChatType,
		CreatedAt:      now.UnixMilli(),
		Payload: messaging.MessageEventPayload{
			ClientMsgID:           ns.ClientMsgID,
			ReceiverID:            ns.ReceiverID,
			GroupID:               ns.GroupID,
			ContentType:           ns.ContentType,
			Content:               json.RawMessage(contentJSON),
			MessageOrigin:         normalizedOrigin(ns.MessageOrigin),
			AgentAccountID:        ns.AgentAccountID,
			TriggerServerMsgID:    ns.TriggerServerMsgID,
			AgentRunID:            ns.AgentRunID,
			AllowRecursiveTrigger: ns.AllowRecursiveTrigger,
			VisibleUserIDs:        visible,
			PayloadHash:           payloadHash,
			SendTime:              now.UnixMilli(),
			TraceID:               trace.TraceID,
			RequestID:             trace.RequestID,
			TraceParent:           trace.TraceParent,
			TraceState:            trace.TraceState,
		},
	}
	if err := l.svcCtx.Producer.PublishEvent(l.ctx, messaging.TopicToTransfer, event); err != nil {
		// 失败优先：Kafka 不可用时显式失败（acks=all），客户端重试（§8.2）。
		l.Errorf("MsgToMQ publish failed conversation_id=%q client_msg_id=%q: %v", ns.ConversationID, ns.ClientMsgID, err)
		return nil, rpcerror.ToStatus(apperror.Internal("message queue unavailable"))
	}

	// ACK 消息体：seq=0（异步分配），其余字段与持久化后等价。
	conversationType, err := model.ConversationTypeValue(ns.ChatType)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	contentType, err := model.ContentTypeValue(ns.ContentType)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	messageOrigin, err := model.MessageOriginValue(ns.MessageOrigin)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	ack := &model.Messages{
		MessageId:             serverMsgIDInt,
		ClientMsgId:           ns.ClientMsgID,
		SenderAccountId:       ns.SenderID,
		ConversationId:        ns.ConversationID,
		Seq:                   0,
		ConversationType:      conversationType,
		ReceiverAccountId:     ns.ReceiverID,
		GroupId:               ns.GroupID,
		ContentType:           contentType,
		Content:               contentJSON,
		MessageOrigin:         messageOrigin,
		AgentAccountId:        ns.AgentAccountID,
		TriggerMessageId:      ns.TriggerServerMsgID,
		AgentRunId:            ns.AgentRunID,
		AllowRecursiveTrigger: ns.AllowRecursiveTrigger,
		PayloadHash:           payloadHash,
		ClientSendTime:        sql.NullTime{Time: now, Valid: true},
		ServerReceivedAt:      now,
	}
	return &msg.SendMessageResponse{Message: messageToPB(ack), Deduplicated: false}, nil
}

func normalizedOrigin(origin string) string {
	if origin == "" {
		return model.MessageOriginHuman
	}
	return origin
}
