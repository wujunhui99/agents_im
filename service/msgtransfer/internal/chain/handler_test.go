package chain

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/wujunhui99/agents_im/pkg/messaging"
)

type fakeSeq struct {
	mu   sync.Mutex
	next map[string]int64
}

func (f *fakeSeq) Malloc(_ context.Context, conversationID string, n int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.next == nil {
		f.next = map[string]int64{}
	}
	first := f.next[conversationID] + 1
	f.next[conversationID] += n
	return first, nil
}

type fakeStore struct {
	mu      sync.Mutex
	dedup   map[string]DedupRecord
	cached  map[string][]messaging.MessageEvent
	hasRead map[string]int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{dedup: map[string]DedupRecord{}, cached: map[string][]messaging.MessageEvent{}, hasRead: map[string]int64{}}
}

func (f *fakeStore) DedupGet(_ context.Context, senderID, clientMsgID string) (*DedupRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if record, ok := f.dedup[senderID+":"+clientMsgID]; ok {
		return &record, nil
	}
	return nil, nil
}

func (f *fakeStore) DedupSet(_ context.Context, senderID, clientMsgID string, record DedupRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := senderID + ":" + clientMsgID
	if _, exists := f.dedup[key]; !exists {
		f.dedup[key] = record
	}
	return nil
}

func (f *fakeStore) CacheMessages(_ context.Context, conversationID string, events []messaging.MessageEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cached[conversationID] = append(f.cached[conversationID], events...)
	return nil
}

func (f *fakeStore) SetHasRead(_ context.Context, conversationID, userID string, seq int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := conversationID + ":" + userID
	if seq > f.hasRead[key] {
		f.hasRead[key] = seq
	}
	return nil
}

type fakeProducer struct {
	mu     sync.Mutex
	events map[string][]messaging.MessageEvent
}

func newFakeProducer() *fakeProducer {
	return &fakeProducer{events: map[string][]messaging.MessageEvent{}}
}

func (f *fakeProducer) PublishEvent(_ context.Context, topic string, event messaging.MessageEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events[topic] = append(f.events[topic], event.Clone())
	return nil
}

func submittedRecord(t *testing.T, conversationID, sender, clientMsgID string) *kgo.Record {
	t.Helper()
	event := messaging.MessageEvent{
		EventID:        "evt-" + clientMsgID,
		EventType:      messaging.EventTypeMessageSubmitted,
		ConversationID: conversationID,
		ServerMsgID:    "srv-" + clientMsgID,
		SenderID:       sender,
		ChatType:       messaging.ChatTypeSingle,
		CreatedAt:      1700000000000,
		Payload: messaging.MessageEventPayload{
			ClientMsgID:    clientMsgID,
			ReceiverID:     "user-b",
			ContentType:    "text",
			Content:        json.RawMessage(`{"text":"hi"}`),
			VisibleUserIDs: []string{sender, "user-b"},
			PayloadHash:    "hash-" + clientMsgID,
			SendTime:       1700000000000,
		},
	}
	raw, err := messaging.MarshalMessageEvent(event)
	if err != nil {
		t.Fatalf("marshal submitted event: %v", err)
	}
	return &kgo.Record{Topic: messaging.TopicToTransfer, Key: []byte(conversationID), Value: raw}
}

func TestHandleBatchAssignsSequentialSeqsAndFansOut(t *testing.T) {
	seq := &fakeSeq{}
	store := newFakeStore()
	producer := newFakeProducer()
	handler, err := NewTransferHandler(seq, store, producer, 4)
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	conv := "single:user-a:user-b"
	records := []*kgo.Record{
		submittedRecord(t, conv, "user-a", "c1"),
		submittedRecord(t, conv, "user-a", "c2"),
		submittedRecord(t, conv, "user-a", "c3"),
	}
	if err := handler.HandleBatch(context.Background(), records); err != nil {
		t.Fatalf("handle batch: %v", err)
	}

	persisted := producer.events[messaging.TopicToPostgres]
	if len(persisted) != 3 {
		t.Fatalf("expected 3 toPostgres events, got %d", len(persisted))
	}
	for i, event := range persisted {
		if event.Seq != int64(i+1) {
			t.Fatalf("expected seq %d, got %d", i+1, event.Seq)
		}
		if event.EventType != messaging.EventTypeMessageAccepted {
			t.Fatalf("expected accepted event, got %s", event.EventType)
		}
		// ACK carries no seq on the Kafka path, so the sender must receive its
		// own push to reconcile the placeholder.
		if !reflect.DeepEqual(event.Payload.ReceiverIDs, []string{"user-a", "user-b"}) {
			t.Fatalf("expected receiver_ids [user-a user-b], got %v", event.Payload.ReceiverIDs)
		}
	}
	for _, topic := range []string{messaging.TopicToPush, messaging.TopicAgentTrigger} {
		if len(producer.events[topic]) != 3 {
			t.Fatalf("expected 3 events on %s, got %d", topic, len(producer.events[topic]))
		}
	}
	if len(store.cached[conv]) != 3 {
		t.Fatalf("expected 3 cached messages, got %d", len(store.cached[conv]))
	}
	if store.hasRead[conv+":user-a"] != 3 {
		t.Fatalf("expected sender has-read 3, got %d", store.hasRead[conv+":user-a"])
	}
}

func TestHandleBatchDedupSkipsReplaysAndInBatchDuplicates(t *testing.T) {
	seq := &fakeSeq{}
	store := newFakeStore()
	producer := newFakeProducer()
	handler, _ := NewTransferHandler(seq, store, producer, 4)

	conv := "single:user-a:user-b"
	first := []*kgo.Record{
		submittedRecord(t, conv, "user-a", "c1"),
		submittedRecord(t, conv, "user-a", "c1"), // in-batch duplicate
	}
	if err := handler.HandleBatch(context.Background(), first); err != nil {
		t.Fatalf("handle first batch: %v", err)
	}
	// Replay of the whole batch (e.g. crash before offset commit).
	if err := handler.HandleBatch(context.Background(), first); err != nil {
		t.Fatalf("handle replayed batch: %v", err)
	}

	if got := len(producer.events[messaging.TopicToPostgres]); got != 1 {
		t.Fatalf("expected exactly 1 toPostgres event after dedup, got %d", got)
	}
	if got := producer.events[messaging.TopicToPostgres][0].Seq; got != 1 {
		t.Fatalf("expected seq 1, got %d", got)
	}
	record, err := store.DedupGet(context.Background(), "user-a", "c1")
	if err != nil || record == nil {
		t.Fatalf("expected dedup record, got %v err %v", record, err)
	}
	if record.Seq != 1 || record.ServerMsgID != "srv-c1" {
		t.Fatalf("unexpected dedup record %+v", record)
	}
}

func TestHandleBatchKeepsConversationsIndependent(t *testing.T) {
	seq := &fakeSeq{}
	store := newFakeStore()
	producer := newFakeProducer()
	handler, _ := NewTransferHandler(seq, store, producer, 4)

	records := make([]*kgo.Record, 0, 10)
	for conversation := 0; conversation < 5; conversation++ {
		conv := fmt.Sprintf("single:a%d:b%d", conversation, conversation)
		records = append(records,
			submittedRecord(t, conv, fmt.Sprintf("a%d", conversation), fmt.Sprintf("c%d-1", conversation)),
			submittedRecord(t, conv, fmt.Sprintf("a%d", conversation), fmt.Sprintf("c%d-2", conversation)),
		)
	}
	if err := handler.HandleBatch(context.Background(), records); err != nil {
		t.Fatalf("handle batch: %v", err)
	}
	perConv := map[string][]int64{}
	for _, event := range producer.events[messaging.TopicToPostgres] {
		perConv[event.ConversationID] = append(perConv[event.ConversationID], event.Seq)
	}
	if len(perConv) != 5 {
		t.Fatalf("expected 5 conversations, got %d", len(perConv))
	}
	for conv, seqs := range perConv {
		if !reflect.DeepEqual(seqs, []int64{1, 2}) {
			t.Fatalf("conversation %s expected seqs [1 2], got %v", conv, seqs)
		}
	}
}

func TestDeriveReceiverIDsAlwaysIncludesSender(t *testing.T) {
	single := messaging.MessageEvent{
		SenderID: "alice",
		ChatType: messaging.ChatTypeSingle,
		Payload:  messaging.MessageEventPayload{ReceiverID: "bob", VisibleUserIDs: []string{"alice", "bob"}},
	}
	if got := deriveReceiverIDs(single); !reflect.DeepEqual(got, []string{"alice", "bob"}) {
		t.Fatalf("single: got %v", got)
	}
	group := messaging.MessageEvent{
		SenderID: "alice",
		ChatType: messaging.ChatTypeGroup,
		Payload:  messaging.MessageEventPayload{VisibleUserIDs: []string{"bob", "carol", "alice"}},
	}
	if got := deriveReceiverIDs(group); !reflect.DeepEqual(got, []string{"alice", "bob", "carol"}) {
		t.Fatalf("group: got %v", got)
	}
}
