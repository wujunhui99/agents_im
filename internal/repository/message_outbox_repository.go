package repository

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

const (
	OutboxEventTypeMessageCreated = "message.created"
	OutboxAggregateTypeMessage    = "message"

	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"
)

type OutboxEvent struct {
	EventID        string          `json:"eventId"`
	EventType      string          `json:"eventType"`
	AggregateType  string          `json:"aggregateType"`
	AggregateID    string          `json:"aggregateId"`
	ConversationID string          `json:"conversationId"`
	ServerMsgID    string          `json:"serverMsgId"`
	Seq            int64           `json:"seq"`
	Payload        json.RawMessage `json:"payload"`
	Status         string          `json:"status"`
	AttemptCount   int             `json:"attemptCount"`
	NextAttemptAt  time.Time       `json:"nextAttemptAt"`
	LockedBy       string          `json:"lockedBy"`
	LockedUntil    time.Time       `json:"lockedUntil"`
	LastError      string          `json:"lastError"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	PublishedAt    time.Time       `json:"publishedAt"`
}

func (e OutboxEvent) Clone() OutboxEvent {
	e.Payload = append(json.RawMessage(nil), e.Payload...)
	return e
}

type MessageCreatedOutboxPayload struct {
	Message        Message  `json:"message"`
	VisibleUserIDs []string `json:"visible_user_ids"`
}

type OutboxFailure struct {
	NextAttemptAt time.Time
	LastError     string
}

type OutboxRepository interface {
	PollPending(ctx context.Context, workerID string, limit int, lockDuration time.Duration) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, eventID string, workerID string) error
	MarkFailed(ctx context.Context, eventID string, workerID string, failure OutboxFailure) error
}

func messageCreatedOutboxPayload(message Message, input CreateMessageInput) (json.RawMessage, error) {
	visibleUserIDs := visibleUserIDs(input)
	sort.Strings(visibleUserIDs)
	payload := MessageCreatedOutboxPayload{
		Message:        message.Clone(),
		VisibleUserIDs: visibleUserIDs,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}
