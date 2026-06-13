package consumer

import (
	"context"
	"sync"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/agent/internal/imadapter"
	"github.com/wujunhui99/agents_im/service/agent/internal/runtime"
	"github.com/wujunhui99/agents_im/service/agent/internal/trigger"
)

type recordingSender struct {
	mu   sync.Mutex
	sent []imadapter.SendAgentMessageRequest
}

func (r *recordingSender) SendAgentMessage(_ context.Context, req imadapter.SendAgentMessageRequest) (imadapter.SendResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, req)
	return imadapter.SendResult{ServerMsgID: "srv-reply"}, nil
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

func TestHandleBatchRunsMockPipelineOnceForReplays(t *testing.T) {
	gen, err := idgen.NewAccountIDGenerator(1)
	if err != nil {
		t.Fatalf("new account id generator: %v", err)
	}
	userID, err := gen.NextString(idgen.AccountTypeUser)
	if err != nil {
		t.Fatalf("mint user id: %v", err)
	}
	agentID, err := gen.NextString(idgen.AccountTypeAgent)
	if err != nil {
		t.Fatalf("mint agent id: %v", err)
	}

	judge, err := trigger.NewJudge(trigger.NewMockHostingStore(nil))
	if err != nil {
		t.Fatalf("new judge: %v", err)
	}
	sender := &recordingSender{}
	pipeline, err := New(judge, runtime.NewMock(), sender)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	batch := []*kgo.Record{triggerRecord(t, "evt-1", userID, agentID)}
	if err := pipeline.HandleBatch(context.Background(), batch); err != nil {
		t.Fatalf("handle batch: %v", err)
	}
	// At-least-once replay of the same event (e.g. crash before offset commit).
	if err := pipeline.HandleBatch(context.Background(), batch); err != nil {
		t.Fatalf("handle replayed batch: %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("expected exactly 1 write-back after replay, got %d", len(sender.sent))
	}
	got := sender.sent[0]
	if got.AgentAccountID != agentID || got.ReceiverID != userID || got.ChatType != messaging.ChatTypeSingle {
		t.Fatalf("unexpected write-back %+v", got)
	}
	if got.TriggerServerMsgID != "srv-evt-1" || got.AgentRunID == "" {
		t.Fatalf("audit chain not threaded: %+v", got)
	}
}

func TestHandleBatchSkipsMalformedAndNonTriggering(t *testing.T) {
	judge, _ := trigger.NewJudge(trigger.NewMockHostingStore(nil))
	sender := &recordingSender{}
	pipeline, err := New(judge, runtime.NewMock(), sender)
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
	if len(sender.sent) != 0 {
		t.Fatalf("expected no write-backs, got %d", len(sender.sent))
	}
}
