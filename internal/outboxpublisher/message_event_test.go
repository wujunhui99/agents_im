package outboxpublisher

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/messaging"

	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestMessageEventFromOutboxBuildsAcceptedEvent(t *testing.T) {
	repo := repository.NewMemoryMessageRepository()
	sent := createTestMessage(t, repo, "client-success", "hello world")

	events, err := repo.PollPending(context.Background(), "reader", 10, time.Minute)
	if err != nil {
		t.Fatalf("poll outbox: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("polled %d outbox events, want 1", len(events))
	}

	event, err := MessageEventFromOutbox(events[0])
	if err != nil {
		t.Fatalf("message event from outbox: %v", err)
	}
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
	if content["text"] != "hello world" {
		t.Fatalf("content text = %q, want hello world", content["text"])
	}
}

func TestMessageEventFromOutboxIncludesAISenderInSingleChatReceiverIDs(t *testing.T) {
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

func TestMessageEventFromOutboxRejectsMalformedPayload(t *testing.T) {
	_, err := MessageEventFromOutbox(malformedOutboxEvent())
	if err == nil || !strings.Contains(err.Error(), "decode message.created outbox payload") {
		t.Fatalf("error = %v, want payload decode error", err)
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

func malformedOutboxEvent() repository.OutboxEvent {
	now := time.Now().UTC()
	return repository.OutboxEvent{
		EventID:        "evt-malformed",
		EventType:      repository.OutboxEventTypeMessageCreated,
		AggregateType:  repository.OutboxAggregateTypeMessage,
		AggregateID:    "msg_000001",
		ConversationID: repository.SingleConversationID("usr_a", "usr_b"),
		ServerMsgID:    "msg_000001",
		Seq:            1,
		Payload:        json.RawMessage(`{bad-json`),
		Status:         repository.OutboxStatusPending,
		NextAttemptAt:  now.Add(-time.Second),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
