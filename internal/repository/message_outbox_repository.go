package repository

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/wujunhui99/agents_im/internal/observability"
)

const (
	OutboxEventTypeMessageCreated int16 = 1
	OutboxAggregateTypeMessage    int16 = 1

	OutboxStatusPending   int16 = 1
	OutboxStatusPublished int16 = 2
	OutboxStatusFailed    int16 = 3
)

type OutboxEvent struct {
	EventID        string          `json:"eventId"`
	EventType      int16           `json:"eventType"`
	AggregateType  int16           `json:"aggregateType"`
	AggregateID    string          `json:"aggregateId"`
	ConversationID string          `json:"conversationId"`
	ServerMsgID    string          `json:"serverMsgId"`
	Seq            int64           `json:"seq"`
	Payload        json.RawMessage `json:"payload"`
	Status         int16           `json:"status"`
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
	Message        Message       `json:"message"`
	VisibleUserIDs []string      `json:"visible_user_ids"`
	TraceContext   TraceMetadata `json:"trace_context,omitempty"`
}

type TraceMetadata struct {
	TraceID     string `json:"trace_id,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
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
		TraceContext:   traceMetadataFromInput(input),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func applyTraceContextToCreateMessageInput(ctx context.Context, input *CreateMessageInput) {
	if input == nil {
		return
	}
	traceContext := observability.TraceContextFromContext(ctx)
	if traceContext.TraceID == "" {
		return
	}
	if input.TraceID == "" {
		input.TraceID = traceContext.TraceID
	}
	if input.RequestID == "" {
		input.RequestID = traceContext.RequestID
	}
	if input.TraceParent == "" {
		input.TraceParent = traceContext.TraceParent
	}
	if input.TraceState == "" {
		input.TraceState = traceContext.TraceState
	}
}

func traceMetadataFromInput(input CreateMessageInput) TraceMetadata {
	return TraceMetadata{
		TraceID:     input.TraceID,
		RequestID:   input.RequestID,
		TraceParent: input.TraceParent,
		TraceState:  input.TraceState,
	}
}
