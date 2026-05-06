package transfer

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestOutboxEventConsumerReceivesMessageAcceptedEnvelope(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	message, _, err := repo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "alice",
		ReceiverID:         "bob",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-1",
		ContentType:        repository.ContentTypeText,
		Content:            "hello outbox",
		ParticipantUserIDs: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	consumer, err := NewOutboxEventConsumer(OutboxEventConsumerConfig{
		Repository: repo,
		WorkerID:   "transfer-test",
	})
	if err != nil {
		t.Fatalf("new outbox consumer: %v", err)
	}

	envelope, err := consumer.Receive(ctx)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if envelope.ID == "" {
		t.Fatal("expected outbox event id")
	}
	if envelope.Event.EventType != EventTypeMessageAccepted {
		t.Fatalf("event type = %q, want %q", envelope.Event.EventType, EventTypeMessageAccepted)
	}
	if envelope.Event.ServerMsgID != message.ServerMsgID || envelope.Event.ConversationID != message.ConversationID || envelope.Event.Seq != message.Seq {
		t.Fatalf("message fields not preserved: envelope=%+v message=%+v", envelope.Event, message)
	}
	if len(envelope.Event.ReceiverIDs) != 1 || envelope.Event.ReceiverIDs[0] != "bob" {
		t.Fatalf("receiver ids = %v, want [bob]", envelope.Event.ReceiverIDs)
	}
	if !strings.Contains(envelope.Event.Content, "hello outbox") {
		t.Fatalf("content = %q, want encoded original text", envelope.Event.Content)
	}
}

func TestOutboxEventConsumerIncludesAISenderAndReceiver(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	trigger, _, err := repo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "bob",
		ReceiverID:         "alice",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-ai-trigger",
		ContentType:        repository.ContentTypeText,
		Content:            "please reply",
		ParticipantUserIDs: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("create trigger message: %v", err)
	}
	message, _, err := repo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "alice",
		ReceiverID:         "bob",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-ai-response",
		ContentType:        repository.ContentTypeText,
		Content:            "AI response",
		MessageOrigin:      repository.MessageOriginAI,
		AgentAccountID:     "alice",
		TriggerServerMsgID: trigger.ServerMsgID,
		AgentRunID:         "run_outbox_consumer_visibility",
		ParticipantUserIDs: []string{"alice", "bob"},
	})
	if err != nil {
		t.Fatalf("create ai message: %v", err)
	}

	consumer, err := NewOutboxEventConsumer(OutboxEventConsumerConfig{
		Repository: repo,
		WorkerID:   "transfer-ai-test",
	})
	if err != nil {
		t.Fatalf("new outbox consumer: %v", err)
	}

	var envelope Envelope
	for {
		next, err := consumer.Receive(ctx)
		if err != nil {
			t.Fatalf("receive: %v", err)
		}
		if next.Event.ServerMsgID == message.ServerMsgID {
			envelope = next
			break
		}
	}
	if len(envelope.Event.ReceiverIDs) != 2 || envelope.Event.ReceiverIDs[0] != "alice" || envelope.Event.ReceiverIDs[1] != "bob" {
		t.Fatalf("receiver ids = %v, want owner and receiver", envelope.Event.ReceiverIDs)
	}
	if envelope.Event.MessageOrigin != repository.MessageOriginAI ||
		envelope.Event.AgentAccountID != "alice" ||
		envelope.Event.TriggerServerMsgID != trigger.ServerMsgID ||
		envelope.Event.AgentRunID != "run_outbox_consumer_visibility" {
		t.Fatalf("ai metadata mismatch: %+v", envelope.Event)
	}
}

func TestOutboxEventConsumerMarksPublishedAndRetry(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryMessageRepository()
	if _, _, err := repo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "alice",
		ReceiverID:         "bob",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-1",
		ContentType:        repository.ContentTypeText,
		Content:            "publish me",
		ParticipantUserIDs: []string{"alice", "bob"},
	}); err != nil {
		t.Fatalf("create message: %v", err)
	}

	consumer, err := NewOutboxEventConsumer(OutboxEventConsumerConfig{
		Repository: repo,
		WorkerID:   "transfer-test",
	})
	if err != nil {
		t.Fatalf("new outbox consumer: %v", err)
	}

	envelope, err := consumer.Receive(ctx)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if err := consumer.MarkSuccessful(ctx, envelope); err != nil {
		t.Fatalf("mark successful: %v", err)
	}
	if _, err := consumer.Receive(ctx); err != ErrNoEvent {
		t.Fatalf("receive after mark successful error = %v, want ErrNoEvent", err)
	}

	if _, _, err := repo.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "alice",
		ReceiverID:         "bob",
		ChatType:           repository.ChatTypeSingle,
		ClientMsgID:        "client-2",
		ContentType:        repository.ContentTypeText,
		Content:            "retry me",
		ParticipantUserIDs: []string{"alice", "bob"},
	}); err != nil {
		t.Fatalf("create retry message: %v", err)
	}

	envelope, err = consumer.Receive(ctx)
	if err != nil {
		t.Fatalf("receive retry envelope: %v", err)
	}
	if err := consumer.MarkRetry(ctx, envelope, RetryDecision{
		NextAttemptAt: time.Now().UTC().Add(time.Minute),
		Reason:        "gateway unavailable",
	}); err != nil {
		t.Fatalf("mark retry: %v", err)
	}
	if _, err := consumer.Receive(ctx); err != ErrNoEvent {
		t.Fatalf("receive while retry delayed error = %v, want ErrNoEvent", err)
	}
}
