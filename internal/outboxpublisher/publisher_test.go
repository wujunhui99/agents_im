package outboxpublisher

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/messaging"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestPublisherPublishesMessageCreatedOutboxEvent(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	sent := createTestMessage(t, repo, "client-success", "hello kafka")
	producer := messaging.NewInMemoryProducer()
	publisher := newTestPublisher(t, repo, producer, WithWorkerID("publisher-success"))

	result, err := publisher.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run publisher: %v", err)
	}
	if result.Polled != 1 || result.Published != 1 || result.Failed != 0 {
		t.Fatalf("unexpected publish result: %+v", result)
	}

	events := producer.Events()
	if len(events) != 1 {
		t.Fatalf("published %d events, want 1", len(events))
	}
	event := events[0]
	if event.EventID != "outbox_000001" ||
		event.EventType != messaging.EventTypeMessageAccepted ||
		event.ConversationID != sent.ConversationID ||
		event.PartitionKey() != sent.ConversationID ||
		event.ServerMsgID != sent.ServerMsgID ||
		event.Seq != sent.Seq ||
		event.SenderID != sent.SenderID ||
		event.ChatType != messaging.ChatTypeSingle {
		t.Fatalf("unexpected message event: %+v", event)
	}
	if len(event.Payload.ReceiverIDs) != 1 || event.Payload.ReceiverIDs[0] != "usr_b" {
		t.Fatalf("receiver ids should contain only the recipient: %+v", event.Payload.ReceiverIDs)
	}
	var content map[string]string
	if err := json.Unmarshal(event.Payload.Content, &content); err != nil {
		t.Fatalf("decode event content: %v", err)
	}
	if content["text"] != "hello kafka" {
		t.Fatalf("content text = %q, want hello kafka", content["text"])
	}

	pending, err := repo.PollPending(ctx, "after-success", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll after publish: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("published outbox event should not remain pending: %+v", pending)
	}
}

func TestPublisherIncludesAISenderInSingleChatReceiverIDs(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	trigger := createTestMessage(t, repo, "client-ai-trigger", "please reply")
	message, deduplicated, err := repo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "usr_a",
		ReceiverID:         "usr_b",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-ai-response",
		ContentType:        repository.ContentTypeText,
		Content:            "AI response",
		MessageOrigin:      repository.MessageOriginAI,
		AgentAccountID:     "usr_a",
		TriggerServerMsgID: trigger.ServerMsgID,
		AgentRunID:         "run_visibility",
		ParticipantUserIDs: []string{"usr_a", "usr_b"},
	})
	if err != nil {
		t.Fatalf("create ai message: %v", err)
	}
	if deduplicated {
		t.Fatal("ai message should not be deduplicated")
	}

	events, err := repo.PollPending(ctx, "publisher-ai-sender", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll outbox: %v", err)
	}
	var aiEvent repository.OutboxEvent
	for _, event := range events {
		if event.ServerMsgID == message.ServerMsgID {
			aiEvent = event
			break
		}
	}
	if aiEvent.EventID == "" {
		t.Fatalf("ai outbox event not found: %+v", events)
	}

	accepted, err := MessageEventFromOutbox(aiEvent)
	if err != nil {
		t.Fatalf("message event from outbox: %v", err)
	}
	if len(accepted.Payload.ReceiverIDs) != 2 ||
		accepted.Payload.ReceiverIDs[0] != "usr_a" ||
		accepted.Payload.ReceiverIDs[1] != "usr_b" {
		t.Fatalf("ai receiver ids = %+v, want owner and receiver", accepted.Payload.ReceiverIDs)
	}
	if accepted.Payload.MessageOrigin != repository.MessageOriginAI ||
		accepted.Payload.AgentAccountID != "usr_a" ||
		accepted.Payload.TriggerServerMsgID != trigger.ServerMsgID ||
		accepted.Payload.AgentRunID != "run_visibility" {
		t.Fatalf("ai metadata mismatch: %+v", accepted.Payload)
	}
}

func TestPublisherMarksPublishErrorRetryable(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	createTestMessage(t, repo, "client-retry", "retry me")
	producer := messaging.NewInMemoryProducer()
	if err := producer.Close(); err != nil {
		t.Fatalf("close producer: %v", err)
	}
	publisher := newTestPublisher(t, repo, producer,
		WithWorkerID("publisher-retry"),
		WithRetryBackoff(0),
		WithMaxAttempts(5),
	)

	result, err := publisher.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run publisher: %v", err)
	}
	if result.Polled != 1 || result.Published != 0 || result.Failed != 1 {
		t.Fatalf("unexpected retry result: %+v", result)
	}
	if result.Err() == nil || !strings.Contains(result.Err().Error(), messaging.ErrProducerClosed.Error()) {
		t.Fatalf("result error = %v, want producer closed", result.Err())
	}

	retried, err := repo.PollPending(ctx, "publisher-retry-next", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll retryable event: %v", err)
	}
	if len(retried) != 1 {
		t.Fatalf("got %d retryable events, want 1", len(retried))
	}
	if retried[0].AttemptCount != 1 || !strings.Contains(retried[0].LastError, messaging.ErrProducerClosed.Error()) {
		t.Fatalf("retry metadata mismatch: %+v", retried[0])
	}
}

func TestPublisherMarksMalformedPayloadFailedForRetry(t *testing.T) {
	ctx := context.Background()
	outbox := newMemoryOutboxRepository(malformedOutboxEvent())
	publisher := newTestPublisher(t, outbox, messaging.NewNoopProducer(),
		WithWorkerID("publisher-malformed"),
		WithRetryBackoff(0),
	)

	result, err := publisher.RunOnce(ctx)
	if err != nil {
		t.Fatalf("run publisher: %v", err)
	}
	if result.Polled != 1 || result.Published != 0 || result.Failed != 1 {
		t.Fatalf("unexpected malformed result: %+v", result)
	}
	if result.Err() == nil || !strings.Contains(result.Err().Error(), "decode message.created outbox payload") {
		t.Fatalf("result error = %v, want payload decode error", result.Err())
	}

	stored := outbox.events[0]
	if stored.Status != repository.OutboxStatusPending ||
		stored.AttemptCount != 1 ||
		stored.LockedBy != "" ||
		stored.LockedUntil != (time.Time{}) ||
		!strings.Contains(stored.LastError, "decode message.created outbox payload") {
		t.Fatalf("malformed event retry metadata mismatch: %+v", stored)
	}
}

func TestPublisherStopsOnContextCancellationWithoutMarkingFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	outbox := newMemoryOutboxRepository(validOutboxEvent("evt-cancel"))
	outbox.pollHook = cancel
	publisher := newTestPublisher(t, outbox, messaging.NewNoopProducer(), WithWorkerID("publisher-cancel"))

	result, err := publisher.RunOnce(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("run publisher error = %v, want context.Canceled", err)
	}
	if result.Polled != 1 || result.Published != 0 || result.Failed != 0 {
		t.Fatalf("unexpected cancellation result: %+v", result)
	}
	stored := outbox.events[0]
	if stored.Status != repository.OutboxStatusPending || stored.AttemptCount != 0 || stored.LastError != "" {
		t.Fatalf("canceled event should not be marked failed: %+v", stored)
	}
}

func createTestMessage(t *testing.T, repo *repository.MemoryMessageRepository, clientMsgID string, content string) repository.Message {
	t.Helper()

	message, deduplicated, err := repo.CreateMessageIdempotent(context.Background(), repository.CreateMessageInput{
		SenderID:    "usr_a",
		ReceiverID:  "usr_b",
		ChatType:    repository.ChatTypeSingle,
		ClientMsgID: clientMsgID,
		ContentType: repository.ContentTypeText,
		Content:     content,
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if deduplicated {
		t.Fatal("test message should not be deduplicated")
	}
	return message
}

func newTestPublisher(t *testing.T, repo repository.OutboxRepository, producer messaging.Producer, opts ...Option) *Publisher {
	t.Helper()

	publisher, err := New(repo, producer, opts...)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	return publisher
}

type memoryOutboxRepository struct {
	events   []repository.OutboxEvent
	pollHook func()
}

func newMemoryOutboxRepository(events ...repository.OutboxEvent) *memoryOutboxRepository {
	return &memoryOutboxRepository{events: events}
}

func (r *memoryOutboxRepository) PollPending(ctx context.Context, workerID string, limit int, lockDuration time.Duration) ([]repository.OutboxEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = len(r.events)
	}
	if lockDuration <= 0 {
		lockDuration = time.Minute
	}

	now := time.Now().UTC()
	lockedUntil := now.Add(lockDuration)
	picked := make([]repository.OutboxEvent, 0, limit)
	for i := range r.events {
		if len(picked) >= limit {
			break
		}
		event := &r.events[i]
		if event.Status != repository.OutboxStatusPending ||
			event.NextAttemptAt.After(now) ||
			(!event.LockedUntil.IsZero() && event.LockedUntil.After(now)) {
			continue
		}
		event.LockedBy = workerID
		event.LockedUntil = lockedUntil
		event.UpdatedAt = now
		picked = append(picked, event.Clone())
	}
	if r.pollHook != nil {
		r.pollHook()
	}
	return picked, nil
}

func (r *memoryOutboxRepository) MarkPublished(_ context.Context, eventID string, workerID string) error {
	event, err := r.lockedEvent(eventID, workerID)
	if err != nil {
		return err
	}
	event.Status = repository.OutboxStatusPublished
	event.LockedBy = ""
	event.LockedUntil = time.Time{}
	event.PublishedAt = time.Now().UTC()
	return nil
}

func (r *memoryOutboxRepository) MarkFailed(_ context.Context, eventID string, workerID string, failure repository.OutboxFailure) error {
	event, err := r.lockedEvent(eventID, workerID)
	if err != nil {
		return err
	}
	event.AttemptCount++
	event.LastError = failure.LastError
	event.LockedBy = ""
	event.LockedUntil = time.Time{}
	if failure.NextAttemptAt.IsZero() {
		event.Status = repository.OutboxStatusFailed
	} else {
		event.Status = repository.OutboxStatusPending
		event.NextAttemptAt = failure.NextAttemptAt
	}
	return nil
}

func (r *memoryOutboxRepository) lockedEvent(eventID string, workerID string) (*repository.OutboxEvent, error) {
	for i := range r.events {
		event := &r.events[i]
		if event.EventID == eventID && event.LockedBy == workerID && event.Status == repository.OutboxStatusPending {
			return event, nil
		}
	}
	return nil, errors.New("locked event not found")
}

func malformedOutboxEvent() repository.OutboxEvent {
	event := validOutboxEvent("evt-malformed")
	event.Payload = json.RawMessage(`{bad-json`)
	return event
}

func validOutboxEvent(eventID string) repository.OutboxEvent {
	now := time.Now().UTC()
	payload, err := json.Marshal(repository.MessageCreatedOutboxPayload{
		Message: repository.Message{
			ServerMsgID:    "msg_000001",
			ClientMsgID:    "client-1",
			ConversationID: repository.SingleConversationID("usr_a", "usr_b"),
			Seq:            1,
			SenderID:       "usr_a",
			ReceiverID:     "usr_b",
			ChatType:       repository.ChatTypeSingle,
			ContentType:    repository.ContentTypeText,
			Content:        "hello",
			SendTime:       now.UnixMilli(),
			CreatedAt:      now.UnixMilli(),
		},
		VisibleUserIDs: []string{"usr_a", "usr_b"},
	})
	if err != nil {
		panic(err)
	}
	return repository.OutboxEvent{
		EventID:        eventID,
		EventType:      repository.OutboxEventTypeMessageCreated,
		AggregateType:  repository.OutboxAggregateTypeMessage,
		AggregateID:    "msg_000001",
		ConversationID: repository.SingleConversationID("usr_a", "usr_b"),
		ServerMsgID:    "msg_000001",
		Seq:            1,
		Payload:        payload,
		Status:         repository.OutboxStatusPending,
		NextAttemptAt:  now.Add(-time.Second),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
