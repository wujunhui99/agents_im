package consumer

import (
	"context"
	"sync"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/trigger"
)

// recordingScheduler is a test double for the orchestrator's ScheduleTrigger
// (idempotency + run + write-back), so the consumer-side judging/mapping can be
// verified without a database or LLM.
type recordingScheduler struct {
	mu        sync.Mutex
	scheduled []orchestrator.AgentTrigger
}

func (r *recordingScheduler) ScheduleTrigger(_ context.Context, trig orchestrator.AgentTrigger) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scheduled = append(r.scheduled, trig)
	return true, nil
}

func triggerRecord(t *testing.T, eventID, sender, receiver string) *kgo.Record {
	t.Helper()
	event := messaging.MessageEvent{
		EventID:        eventID,
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: "single:" + sender + ":" + receiver,
		ServerMsgID:    "srv-" + eventID,
		Seq:            1,
		SenderID:       sender,
		ChatType:       messaging.ChatTypeSingle,
		CreatedAt:      1700000000000,
		Payload: messaging.MessageEventPayload{
			ClientMsgID:   "c-" + eventID,
			ReceiverID:    receiver,
			ContentType:   "text",
			Content:       []byte(`{"text":"hello agent"}`),
			MessageOrigin: messaging.MessageOriginHuman,
			ReceiverIDs:   []string{sender, receiver},
		},
	}
	raw, err := messaging.MarshalMessageEvent(event)
	if err != nil {
		t.Fatalf("marshal trigger event: %v", err)
	}
	return &kgo.Record{Topic: messaging.TopicAgentTrigger, Key: []byte(event.ConversationID), Value: raw}
}

func TestHandleBatchSchedulesAgentInboxTrigger(t *testing.T) {
	gen, err := idgen.NewAccountIDGenerator(1)
	if err != nil {
		t.Fatalf("new account id generator: %v", err)
	}
	userID, err := gen.NextString(idgen.FacetHuman)
	if err != nil {
		t.Fatalf("mint user id: %v", err)
	}
	agentID, err := gen.NextString(idgen.FacetAgent)
	if err != nil {
		t.Fatalf("mint agent id: %v", err)
	}

	judge, err := trigger.NewJudge(trigger.NewMockHostingStore(nil))
	if err != nil {
		t.Fatalf("new judge: %v", err)
	}
	scheduler := &recordingScheduler{}
	pipeline, err := New(judge, scheduler)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	batch := []*kgo.Record{triggerRecord(t, "evt-1", userID, agentID)}
	if err := pipeline.HandleBatch(context.Background(), batch); err != nil {
		t.Fatalf("handle batch: %v", err)
	}

	if len(scheduler.scheduled) != 1 {
		t.Fatalf("expected exactly 1 scheduled trigger, got %d", len(scheduler.scheduled))
	}
	got := scheduler.scheduled[0]
	if got.AgentUserID != agentID || got.RequestingUserID != userID || got.ConversationType != messaging.ChatTypeSingle {
		t.Fatalf("unexpected trigger routing %+v", got)
	}
	if got.TriggerType != orchestrator.TriggerTypeUserPrivateMessage {
		t.Fatalf("unexpected trigger_type %q", got.TriggerType)
	}
	if got.RequestID != "evt-1:"+agentID || got.TriggerMessageID != "srv-evt-1" || got.TriggerSeq != 1 {
		t.Fatalf("audit/idempotency chain not threaded: %+v", got)
	}
}

func TestHandleBatchSkipsMalformedAndNonTriggering(t *testing.T) {
	judge, _ := trigger.NewJudge(trigger.NewMockHostingStore(nil))
	scheduler := &recordingScheduler{}
	pipeline, err := New(judge, scheduler)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	records := []*kgo.Record{
		{Topic: messaging.TopicAgentTrigger, Value: []byte("not-json")},
		triggerRecord(t, "evt-2", "user-a", "user-b"), // legacy ids, no agent bits
	}
	if err := pipeline.HandleBatch(context.Background(), records); err != nil {
		t.Fatalf("handle batch must commit despite malformed records: %v", err)
	}
	if len(scheduler.scheduled) != 0 {
		t.Fatalf("expected no scheduled triggers, got %d", len(scheduler.scheduled))
	}
}
