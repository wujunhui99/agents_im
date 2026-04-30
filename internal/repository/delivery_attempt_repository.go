package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

const (
	DeliveryStatusAccepted  = "accepted"
	DeliveryStatusPublished = "published"
	DeliveryStatusDelivered = "delivered"
	DeliveryStatusOffline   = "offline"
	DeliveryStatusFailed    = "failed"
)

const maxDeliveryAttemptLastErrorLength = 1024

type DeliveryAttempt struct {
	ServerMsgID     string    `json:"serverMsgId"`
	ConversationID  string    `json:"conversationId"`
	RecipientUserID string    `json:"recipientUserId"`
	Status          string    `json:"status"`
	AttemptCount    int       `json:"attemptCount"`
	LastError       string    `json:"lastError"`
	NextRetryAt     time.Time `json:"nextRetryAt"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (a DeliveryAttempt) Clone() DeliveryAttempt {
	return a
}

type CreateDeliveryAttemptInput struct {
	ServerMsgID     string
	ConversationID  string
	RecipientUserID string
}

type RecordDeliveryAttemptInput struct {
	ServerMsgID     string
	ConversationID  string
	RecipientUserID string
	Status          string
	AttemptCount    int
	LastError       string
	NextRetryAt     time.Time
}

type DeliveryAttemptRepository interface {
	CreateDeliveryAttemptsAccepted(ctx context.Context, attempts []CreateDeliveryAttemptInput) error
	MarkDeliveryAttemptsPublished(ctx context.Context, serverMsgID string, recipientUserIDs []string) error
	RecordDeliveryAttemptResult(ctx context.Context, input RecordDeliveryAttemptInput) error
	ListDeliveryAttemptsByMessage(ctx context.Context, serverMsgID string) ([]DeliveryAttempt, error)
}

func normalizeCreateDeliveryAttemptInput(input CreateDeliveryAttemptInput) (CreateDeliveryAttemptInput, error) {
	input.ServerMsgID = strings.TrimSpace(input.ServerMsgID)
	input.ConversationID = strings.TrimSpace(input.ConversationID)
	input.RecipientUserID = strings.TrimSpace(input.RecipientUserID)
	if input.ServerMsgID == "" {
		return CreateDeliveryAttemptInput{}, apperror.InvalidArgument("server_msg_id is required")
	}
	if input.ConversationID == "" {
		return CreateDeliveryAttemptInput{}, apperror.InvalidArgument("conversation_id is required")
	}
	if input.RecipientUserID == "" {
		return CreateDeliveryAttemptInput{}, apperror.InvalidArgument("recipient_user_id is required")
	}
	return input, nil
}

func normalizeRecordDeliveryAttemptInput(input RecordDeliveryAttemptInput) (RecordDeliveryAttemptInput, error) {
	createInput, err := normalizeCreateDeliveryAttemptInput(CreateDeliveryAttemptInput{
		ServerMsgID:     input.ServerMsgID,
		ConversationID:  input.ConversationID,
		RecipientUserID: input.RecipientUserID,
	})
	if err != nil {
		return RecordDeliveryAttemptInput{}, err
	}
	input.ServerMsgID = createInput.ServerMsgID
	input.ConversationID = createInput.ConversationID
	input.RecipientUserID = createInput.RecipientUserID
	input.Status = strings.TrimSpace(input.Status)
	if !isDeliveryStatus(input.Status) {
		return RecordDeliveryAttemptInput{}, apperror.InvalidArgument("delivery attempt status is invalid")
	}
	if input.AttemptCount <= 0 {
		input.AttemptCount = 1
	}
	input.LastError = trimDeliveryLastError(input.LastError)
	if !input.NextRetryAt.IsZero() {
		input.NextRetryAt = input.NextRetryAt.UTC()
	}
	return input, nil
}

func isDeliveryStatus(status string) bool {
	switch status {
	case DeliveryStatusAccepted,
		DeliveryStatusPublished,
		DeliveryStatusDelivered,
		DeliveryStatusOffline,
		DeliveryStatusFailed:
		return true
	default:
		return false
	}
}

func trimDeliveryLastError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxDeliveryAttemptLastErrorLength {
		return value
	}
	return value[:maxDeliveryAttemptLastErrorLength]
}

func DeliveryRecipientUserIDs(input CreateMessageInput) []string {
	seen := make(map[string]struct{})
	add := func(userID string) {
		userID = strings.TrimSpace(userID)
		if userID == "" || userID == strings.TrimSpace(input.SenderID) {
			return
		}
		seen[userID] = struct{}{}
	}

	switch input.ChatType {
	case ChatTypeSingle:
		add(input.ReceiverID)
	case ChatTypeGroup:
		for _, userID := range input.ParticipantUserIDs {
			add(userID)
		}
	}

	recipients := make([]string, 0, len(seen))
	for userID := range seen {
		recipients = append(recipients, userID)
	}
	sort.Strings(recipients)
	return recipients
}

func deliveryAttemptRecipientUserIDs(input CreateMessageInput) []string {
	return DeliveryRecipientUserIDs(input)
}
