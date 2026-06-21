package pusher

import (
	"context"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	pushgateway "github.com/wujunhui99/agents_im/service/push/internal/gateway"
)

type fakeBroadcaster struct {
	result  pushgateway.Result
	err     error
	called  int
	lastReq pushgateway.PushRequest
}

func (f *fakeBroadcaster) Broadcast(_ context.Context, req pushgateway.PushRequest) (pushgateway.Result, error) {
	f.called++
	f.lastReq = req
	return f.result, f.err
}

type fakeProducer struct {
	topic  string
	events []messaging.MessageEvent
}

func (f *fakeProducer) PublishEvent(_ context.Context, topic string, event messaging.MessageEvent) error {
	f.topic = topic
	f.events = append(f.events, event)
	return nil
}

func acceptedRecord(t *testing.T, receivers []string) *kgo.Record {
	t.Helper()
	event := messaging.MessageEvent{
		EventID:        "evt-1",
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: "conv-1",
		ServerMsgID:    "100",
		Seq:            1,
		SenderID:       "sender",
		ChatType:       messaging.ChatTypeSingle,
		CreatedAt:      1700000000000,
		Payload: messaging.MessageEventPayload{
			ClientMsgID: "cmid-1",
			ReceiverID:  "receiver",
			ContentType: "text",
			ReceiverIDs: receivers,
		},
	}
	value, err := messaging.MarshalMessageEvent(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return &kgo.Record{Topic: messaging.TopicToPush, Value: value}
}

func TestOnlineHandlerProducesOfflineForMissedRecipients(t *testing.T) {
	broadcaster := &fakeBroadcaster{result: pushgateway.Result{
		Delivered:      map[string]bool{"sender": true},
		OfflineUserIDs: []string{"receiver"},
	}}
	producer := &fakeProducer{}
	handler, err := NewOnlineHandler(broadcaster, producer)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	if err := handler.HandleBatch(context.Background(), []*kgo.Record{acceptedRecord(t, []string{"sender", "receiver"})}); err != nil {
		t.Fatalf("handle batch: %v", err)
	}
	if broadcaster.called != 1 {
		t.Fatalf("expected 1 broadcast, got %d", broadcaster.called)
	}
	if len(producer.events) != 1 {
		t.Fatalf("expected 1 offline event, got %d", len(producer.events))
	}
	if producer.topic != messaging.TopicToOfflinePush {
		t.Fatalf("expected topic %s, got %s", messaging.TopicToOfflinePush, producer.topic)
	}
	offline := producer.events[0]
	if len(offline.Payload.ReceiverIDs) != 1 || offline.Payload.ReceiverIDs[0] != "receiver" {
		t.Fatalf("expected offline receiver_ids=[receiver], got %v", offline.Payload.ReceiverIDs)
	}
	if offline.Payload.ReceiverID != "" {
		t.Fatalf("expected single receiver_id cleared on offline event, got %q", offline.Payload.ReceiverID)
	}
}

func TestOnlineHandlerNoOfflineWhenAllDelivered(t *testing.T) {
	broadcaster := &fakeBroadcaster{result: pushgateway.Result{
		Delivered:      map[string]bool{"sender": true, "receiver": true},
		OfflineUserIDs: nil,
	}}
	producer := &fakeProducer{}
	handler, _ := NewOnlineHandler(broadcaster, producer)

	if err := handler.HandleBatch(context.Background(), []*kgo.Record{acceptedRecord(t, []string{"sender", "receiver"})}); err != nil {
		t.Fatalf("handle batch: %v", err)
	}
	if len(producer.events) != 0 {
		t.Fatalf("expected no offline events, got %d", len(producer.events))
	}
}

func TestOnlineHandlerBroadcastErrorRetriesBatch(t *testing.T) {
	broadcaster := &fakeBroadcaster{err: errors.New("gateway down")}
	producer := &fakeProducer{}
	handler, _ := NewOnlineHandler(broadcaster, producer)

	err := handler.HandleBatch(context.Background(), []*kgo.Record{acceptedRecord(t, []string{"sender", "receiver"})})
	if err == nil {
		t.Fatal("expected error so the batch is retried (at-least-once)")
	}
	if len(producer.events) != 0 {
		t.Fatalf("must not produce offline on broadcast failure, got %d", len(producer.events))
	}
}

func TestOnlineHandlerSkipsMalformedRecord(t *testing.T) {
	broadcaster := &fakeBroadcaster{}
	producer := &fakeProducer{}
	handler, _ := NewOnlineHandler(broadcaster, producer)

	// Malformed record must be skipped (return nil) — not wedge the partition.
	if err := handler.HandleBatch(context.Background(), []*kgo.Record{{Topic: messaging.TopicToPush, Value: []byte("{not json")}}); err != nil {
		t.Fatalf("malformed record should be skipped, got %v", err)
	}
	if broadcaster.called != 0 {
		t.Fatalf("malformed record must not broadcast")
	}
}
