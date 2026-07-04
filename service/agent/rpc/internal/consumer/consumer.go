// Package consumer wires the agent.trigger.v1 pipeline (04-agent §4.2, D15
// step ③/④): judge → orchestrator.ScheduleTrigger. Per-event errors are logged
// and the batch still commits — trigger processing must never wedge the topic
// (same semantics as the retired msg-rpc 回流 consumer / legacy
// fireMessageCreatedHook). Idempotency is durable (agent_triggers ledger via
// TryStartAgentTrigger keyed on RequestID), so replays after a crash before
// offset commit do not double-reply.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/trigger"
)

// Scheduler accepts one already-judged agent trigger: idempotency gate +
// group-membership authorization + read-marking + async run/write-back.
// *orchestrator.ConversationHostingService implements it (ScheduleTrigger).
type Scheduler interface {
	ScheduleTrigger(ctx context.Context, trigger orchestrator.AgentTrigger) (bool, error)
}

// Coalescer 承接直连 agent 私聊（KindAgentInbox·single，#686）：入站消息落 agent 自有 conversation
// store 并合流回复，取代 ScheduleTrigger 的 per-event 立即执行。*orchestrator.PrivateChatCoalescer
// 实现它。nil 时私聊回退 Scheduler（旧 msg-rpc 历史路径），保持行为不变。
type Coalescer interface {
	OnUserMessage(ctx context.Context, base orchestrator.AgentTrigger, message orchestrator.PrivateUserMessage) error
}

type Consumer struct {
	judge     *trigger.Judge
	scheduler Scheduler
	coalescer Coalescer
}

func New(judge *trigger.Judge, scheduler Scheduler, coalescer Coalescer) (*Consumer, error) {
	if judge == nil || scheduler == nil {
		return nil, fmt.Errorf("agent consumer requires judge and scheduler")
	}
	return &Consumer{judge: judge, scheduler: scheduler, coalescer: coalescer}, nil
}

// HandleBatch is the kgo poll callback: returning nil commits offsets.
func (c *Consumer) HandleBatch(ctx context.Context, records []*kgo.Record) error {
	for _, record := range records {
		event, err := messaging.UnmarshalMessageEvent(record.Value)
		if err != nil {
			logx.WithContext(ctx).Errorf("agent: drop malformed trigger record topic=%s partition=%d offset=%d: %v",
				record.Topic, record.Partition, record.Offset, err)
			continue
		}
		if event.EventType != messaging.EventTypeMessageAccepted {
			continue
		}
		c.handleEvent(ctx, event)
	}
	return nil
}

func (c *Consumer) handleEvent(ctx context.Context, event messaging.MessageEvent) {
	triggers, err := c.judge.Evaluate(ctx, event)
	if err != nil {
		logx.WithContext(ctx).Errorf("agent: judge failed event_id=%q conversation_id=%q seq=%d: %v",
			event.EventID, event.ConversationID, event.Seq, err)
		return
	}
	for _, trig := range triggers {
		agentTrigger, err := agentTriggerFromJudged(trig)
		if err != nil {
			logx.WithContext(ctx).Errorf("agent: build trigger kind=%s agent=%q event_id=%q: %v",
				trig.Kind, trig.AgentAccountID, trig.Event.EventID, err)
			continue
		}
		// 直连 agent 私聊（KindAgentInbox·single）：走 conversation store 合流器（#686），入站消息
		// 直写 store、合流回复，不再 per-event 立即 ScheduleTrigger、也不同步调 msg-rpc 拉历史。
		// 托管/群聊仍走 ScheduleTrigger（agent_triggers 台账 + msg-rpc 历史）。coalescer nil 时回退。
		if c.coalescer != nil && trig.Kind == trigger.KindAgentInbox && agentTrigger.ConversationType == orchestrator.ConversationTypeSingle {
			message := orchestrator.PrivateUserMessage{
				Seq:         trig.Event.Seq,
				ServerMsgID: trig.Event.ServerMsgID,
				Text:        decodeMessageText(trig.Event.Payload.ContentType, trig.Event.Payload.Content),
				CreatedAtMs: trig.Event.CreatedAt,
			}
			if err := c.coalescer.OnUserMessage(ctx, agentTrigger, message); err != nil {
				logx.WithContext(ctx).Errorf("agent: coalesce failed agent=%q event_id=%q: %v",
					trig.AgentAccountID, trig.Event.EventID, err)
			}
			continue
		}
		if _, err := c.scheduler.ScheduleTrigger(ctx, agentTrigger); err != nil {
			logx.WithContext(ctx).Errorf("agent: schedule failed kind=%s agent=%q event_id=%q: %v",
				trig.Kind, trig.AgentAccountID, trig.Event.EventID, err)
			continue
		}
	}
}

// decodeMessageText 把 Kafka 事件 payload 的 content（text 存成 {"text":...}）还原成纯文本，
// 喂 conversation store 的 pending。非文本消息给占位符（与 msg-rpc 历史路径 hostingRuntimeText
// 一致），保证合流后当轮 user 输入非空。
func decodeMessageText(contentType string, raw json.RawMessage) string {
	switch strings.TrimSpace(contentType) {
	case orchestrator.MessageContentTypeImage:
		return "[图片消息]"
	case orchestrator.MessageContentTypeFile:
		return "[文件消息]"
	}
	var body struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &body); err == nil {
		if text := strings.TrimSpace(body.Text); text != "" {
			return text
		}
	}
	if text := strings.TrimSpace(string(raw)); text != "" {
		return text
	}
	return "[非文本消息]"
}

// agentTriggerFromJudged maps a judged trigger.Trigger (recursion gate /
// agent-inbox / hosting decision already made) onto the orchestrator's
// AgentTrigger. Field conventions mirror the retired in-process
// BuildMessageCreatedTriggers so audit / idempotency keys stay stable:
// RequestID = "<event_id>:<agent_account_id>". The prompt context itself is
// loaded from the message history (PG) by the runtime request builder; the
// trigger only carries routing + provenance metadata.
func agentTriggerFromJudged(trig trigger.Trigger) (orchestrator.AgentTrigger, error) {
	event := trig.Event
	triggerType, err := messageTriggerType(event.ChatType)
	if err != nil {
		return orchestrator.AgentTrigger{}, err
	}
	recursive := event.Payload.MessageOrigin == messaging.MessageOriginAI
	sourceAgentUserID := ""
	if recursive {
		sourceAgentUserID = event.SenderID
	}
	return orchestrator.AgentTrigger{
		RequestID:          event.EventID + ":" + trig.AgentAccountID,
		EventID:            event.EventID,
		TraceID:            event.Payload.TraceID,
		TriggerType:        triggerType,
		AgentUserID:        trig.AgentAccountID,
		RequestingUserID:   event.SenderID,
		ConversationID:     event.ConversationID,
		ConversationType:   event.ChatType,
		TriggerMessageID:   event.ServerMsgID,
		TriggerSeq:         event.Seq,
		ReplyToMessageID:   event.ServerMsgID,
		RecursiveTrigger:   recursive,
		SourceAgentRunID:   event.Payload.AgentRunID,
		SourceAgentUserID:  sourceAgentUserID,
		SourceMessageID:    event.ServerMsgID,
		SourceMessageSeq:   event.Seq,
		SourceContentType:  event.Payload.ContentType,
		TargetAgentUserIDs: []string{trig.AgentAccountID},
	}, nil
}

func messageTriggerType(chatType string) (string, error) {
	switch chatType {
	case messaging.ChatTypeSingle:
		return orchestrator.TriggerTypeUserPrivateMessage, nil
	case messaging.ChatTypeGroup:
		return orchestrator.TriggerTypeGroupMention, nil
	default:
		return "", fmt.Errorf("unsupported chat_type %q", chatType)
	}
}
