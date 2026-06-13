// Package consumer wires the agent.trigger.v1 pipeline: judge → runtime →
// imadapter (04-agent §4.2). Per-event errors are logged and the batch still
// commits — trigger processing must never wedge the topic (same semantics as
// the legacy fireMessageCreatedHook and the transitional msg-rpc consumer).
package consumer

import (
	"context"
	"fmt"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/agent/internal/imadapter"
	"github.com/wujunhui99/agents_im/service/agent/internal/runtime"
	"github.com/wujunhui99/agents_im/service/agent/internal/trigger"
)

// seenCap bounds the in-memory idempotency set. Scaffold-only: the real
// implementation records trigger event_id in the agent run audit table (D15)
// so replays survive restarts.
const seenCap = 65536

type Consumer struct {
	judge   *trigger.Judge
	runtime runtime.Runtime
	sender  imadapter.MessageSender

	mu        sync.Mutex
	seen      map[string]struct{}
	seenOrder []string
}

func New(judge *trigger.Judge, rt runtime.Runtime, sender imadapter.MessageSender) (*Consumer, error) {
	if judge == nil || rt == nil || sender == nil {
		return nil, fmt.Errorf("agent consumer requires judge, runtime and sender")
	}
	return &Consumer{judge: judge, runtime: rt, sender: sender, seen: make(map[string]struct{})}, nil
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
		idempotencyKey := trig.Event.EventID + ":" + trig.AgentAccountID
		if c.alreadyRan(idempotencyKey) {
			continue
		}
		if err := c.runTrigger(ctx, trig); err != nil {
			logx.WithContext(ctx).Errorf("agent: run failed kind=%s agent=%q event_id=%q: %v",
				trig.Kind, trig.AgentAccountID, trig.Event.EventID, err)
			continue
		}
		c.markRan(idempotencyKey)
	}
}

func (c *Consumer) runTrigger(ctx context.Context, trig trigger.Trigger) error {
	event := trig.Event
	result, err := c.runtime.Run(ctx, runtime.RunRequest{
		TriggerEventID:     event.EventID,
		TriggerKind:        string(trig.Kind),
		AgentAccountID:     trig.AgentAccountID,
		ConversationID:     event.ConversationID,
		ChatType:           event.ChatType,
		Seq:                event.Seq,
		TriggerServerMsgID: event.ServerMsgID,
		SenderID:           event.SenderID,
		ContentType:        event.Payload.ContentType,
		Content:            event.Payload.Content,
	})
	if err != nil {
		return fmt.Errorf("runtime: %w", err)
	}

	send := imadapter.SendAgentMessageRequest{
		AgentAccountID:     trig.AgentAccountID,
		ChatType:           event.ChatType,
		ClientMsgID:        result.RunID,
		ContentType:        result.ReplyContentType,
		Content:            result.ReplyContent,
		TriggerServerMsgID: event.ServerMsgID,
		AgentRunID:         result.RunID,
	}
	switch event.ChatType {
	case messaging.ChatTypeSingle:
		// The agent replies to the human peer — for an inbox trigger that is the
		// original sender (the agent was the receiver).
		send.ReceiverID = event.SenderID
	case messaging.ChatTypeGroup:
		send.GroupID = event.Payload.GroupID
	}
	if _, err := c.sender.SendAgentMessage(ctx, send); err != nil {
		return fmt.Errorf("imadapter: %w", err)
	}
	return nil
}

func (c *Consumer) alreadyRan(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.seen[key]
	return ok
}

func (c *Consumer) markRan(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.seenOrder) >= seenCap {
		oldest := c.seenOrder[0]
		c.seenOrder = c.seenOrder[1:]
		delete(c.seen, oldest)
	}
	c.seen[key] = struct{}{}
	c.seenOrder = append(c.seenOrder, key)
}
