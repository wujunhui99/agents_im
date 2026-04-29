package repository

import (
	"context"
	"testing"
	"time"
)

func TestMemoryMessageRepositoryCreatesAcceptedDeliveryAttempt(t *testing.T) {
	repo := NewMemoryMessageRepository()
	ctx := context.Background()

	message, deduplicated, err := repo.CreateMessageIdempotent(ctx, CreateMessageInput{
		SenderID:           "usr_sender",
		ReceiverID:         "usr_receiver",
		ChatType:           ChatTypeSingle,
		ClientMsgID:        "client-delivery-accepted",
		ContentType:        ContentTypeText,
		Content:            "hello",
		ParticipantUserIDs: []string{"usr_sender", "usr_receiver"},
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	if deduplicated {
		t.Fatal("first message should not be deduplicated")
	}

	attempts, err := repo.ListDeliveryAttemptsByMessage(ctx, message.ServerMsgID)
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("attempts = %d, want 1: %+v", len(attempts), attempts)
	}
	attempt := attempts[0]
	if attempt.ConversationID != message.ConversationID ||
		attempt.RecipientUserID != "usr_receiver" ||
		attempt.Status != DeliveryStatusAccepted ||
		attempt.AttemptCount != 0 {
		t.Fatalf("accepted attempt mismatch: %+v", attempt)
	}
}

func TestMemoryMessageRepositoryMarksDeliveryAttemptPublishedWithOutbox(t *testing.T) {
	repo := NewMemoryMessageRepository()
	ctx := context.Background()

	message, _, err := repo.CreateMessageIdempotent(ctx, CreateMessageInput{
		SenderID:           "usr_sender",
		ReceiverID:         "usr_receiver",
		ChatType:           ChatTypeSingle,
		ClientMsgID:        "client-delivery-published",
		ContentType:        ContentTypeText,
		Content:            "hello",
		ParticipantUserIDs: []string{"usr_sender", "usr_receiver"},
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	events, err := repo.PollPending(ctx, "publisher-1", 1, time.Minute)
	if err != nil {
		t.Fatalf("poll outbox: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if err := repo.MarkPublished(ctx, events[0].EventID, "publisher-1"); err != nil {
		t.Fatalf("mark outbox published: %v", err)
	}

	attempts, err := repo.ListDeliveryAttemptsByMessage(ctx, message.ServerMsgID)
	if err != nil {
		t.Fatalf("list delivery attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].Status != DeliveryStatusPublished {
		t.Fatalf("attempt should be published after outbox publish: %+v", attempts)
	}
}

func TestMemoryMessageRepositoryRecordsDeliveryAttemptResults(t *testing.T) {
	repo := NewMemoryMessageRepository()
	ctx := context.Background()

	message, _, err := repo.CreateMessageIdempotent(ctx, CreateMessageInput{
		SenderID:           "usr_sender",
		ReceiverID:         "usr_receiver",
		ChatType:           ChatTypeSingle,
		ClientMsgID:        "client-delivery-result",
		ContentType:        ContentTypeText,
		Content:            "hello",
		ParticipantUserIDs: []string{"usr_sender", "usr_receiver"},
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	if err := repo.RecordDeliveryAttemptResult(ctx, RecordDeliveryAttemptInput{
		ServerMsgID:     message.ServerMsgID,
		ConversationID:  message.ConversationID,
		RecipientUserID: "usr_receiver",
		Status:          DeliveryStatusDelivered,
		AttemptCount:    1,
	}); err != nil {
		t.Fatalf("record delivered: %v", err)
	}
	attempts, err := repo.ListDeliveryAttemptsByMessage(ctx, message.ServerMsgID)
	if err != nil {
		t.Fatalf("list after delivered: %v", err)
	}
	if attempts[0].Status != DeliveryStatusDelivered || attempts[0].AttemptCount != 1 || !attempts[0].NextRetryAt.IsZero() {
		t.Fatalf("delivered attempt mismatch: %+v", attempts[0])
	}

	nextRetryAt := time.Now().UTC().Add(time.Minute)
	if err := repo.RecordDeliveryAttemptResult(ctx, RecordDeliveryAttemptInput{
		ServerMsgID:     message.ServerMsgID,
		ConversationID:  message.ConversationID,
		RecipientUserID: "usr_receiver",
		Status:          DeliveryStatusFailed,
		AttemptCount:    2,
		LastError:       "gateway unavailable",
		NextRetryAt:     nextRetryAt,
	}); err != nil {
		t.Fatalf("record failed: %v", err)
	}
	attempts, err = repo.ListDeliveryAttemptsByMessage(ctx, message.ServerMsgID)
	if err != nil {
		t.Fatalf("list after failed: %v", err)
	}
	if attempts[0].Status != DeliveryStatusFailed ||
		attempts[0].AttemptCount != 2 ||
		attempts[0].LastError != "gateway unavailable" ||
		attempts[0].NextRetryAt.IsZero() {
		t.Fatalf("failed retry attempt mismatch: %+v", attempts[0])
	}

	if err := repo.RecordDeliveryAttemptResult(ctx, RecordDeliveryAttemptInput{
		ServerMsgID:     message.ServerMsgID,
		ConversationID:  message.ConversationID,
		RecipientUserID: "usr_receiver",
		Status:          DeliveryStatusOffline,
		AttemptCount:    3,
	}); err != nil {
		t.Fatalf("record offline: %v", err)
	}
	attempts, err = repo.ListDeliveryAttemptsByMessage(ctx, message.ServerMsgID)
	if err != nil {
		t.Fatalf("list after offline: %v", err)
	}
	if attempts[0].Status != DeliveryStatusOffline ||
		attempts[0].AttemptCount != 3 ||
		!attempts[0].NextRetryAt.IsZero() {
		t.Fatalf("offline attempt mismatch: %+v", attempts[0])
	}
}
