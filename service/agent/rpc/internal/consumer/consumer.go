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
	"fmt"

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

type Consumer struct {
	judge     *trigger.Judge
	scheduler Scheduler
}

func New(judge *trigger.Judge, scheduler Scheduler) (*Consumer, error) {
	if judge == nil || scheduler == nil {
		return nil, fmt.Errorf("agent consumer requires judge and scheduler")
	}
	return &Consumer{judge: judge, scheduler: scheduler}, nil
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
		if _, err := c.scheduler.ScheduleTrigger(ctx, agentTrigger); err != nil {
			logx.WithContext(ctx).Errorf("agent: schedule failed kind=%s agent=%q event_id=%q: %v",
				trig.Kind, trig.AgentAccountID, trig.Event.EventID, err)
			continue
		}
	}
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
