package transfer

import (
	"context"
	"errors"
	"time"
)

var ErrNoEvent = errors.New("message transfer: no event available")

type EventConsumer interface {
	Receive(ctx context.Context) (Envelope, error)
	MarkSuccessful(ctx context.Context, envelope Envelope) error
	MarkRetry(ctx context.Context, envelope Envelope, decision RetryDecision) error
	MarkFailed(ctx context.Context, envelope Envelope, result ProcessResult) error
}

type DeliveryDispatcher interface {
	Dispatch(ctx context.Context, envelope Envelope) DispatchResult
}

type IdempotencyStore interface {
	HasProcessed(ctx context.Context, key string) (bool, error)
	MarkProcessed(ctx context.Context, key string) error
}

type DeliveryAttemptRecorder interface {
	RecordDeliveryResults(ctx context.Context, envelope Envelope, result ProcessResult) error
}

type ResultStatus string

const (
	StatusNoEvent   ResultStatus = "no_event"
	StatusSucceeded ResultStatus = "succeeded"
	StatusRetryable ResultStatus = "retryable"
	StatusFailed    ResultStatus = "failed"
	StatusStopped   ResultStatus = "stopped"
)

type RecipientDeliveryStatus string

const (
	RecipientDeliveryDelivered RecipientDeliveryStatus = "delivered"
	RecipientDeliveryOffline   RecipientDeliveryStatus = "offline"
	RecipientDeliveryFailed    RecipientDeliveryStatus = "failed"
)

type DispatchResult struct {
	Status           ResultStatus
	Retryable        bool
	RetryAfter       time.Duration
	Err              error
	DeliveredUserIDs []string
	RecipientResults []RecipientDeliveryResult
}

type RetryDecision struct {
	Attempt       int
	MaxAttempts   int
	Backoff       time.Duration
	NextAttemptAt time.Time
	Reason        string
}

type ProcessResult struct {
	Status           ResultStatus
	EventID          string
	IdempotencyKey   string
	Attempt          int
	MaxAttempts      int
	Retryable        bool
	RetryAfter       time.Duration
	DeliveredUserIDs []string
	Err              error
	RecipientResults []RecipientDeliveryResult
}

type RecipientDeliveryResult struct {
	UserID string
	Status RecipientDeliveryStatus
	Error  string
}

func cloneRecipientDeliveryResults(results []RecipientDeliveryResult) []RecipientDeliveryResult {
	return append([]RecipientDeliveryResult(nil), results...)
}

func DispatchSucceeded(deliveredUserIDs ...string) DispatchResult {
	return DispatchResult{
		Status:           StatusSucceeded,
		DeliveredUserIDs: append([]string(nil), deliveredUserIDs...),
	}
}

func DispatchRetryable(err error, retryAfter time.Duration) DispatchResult {
	return DispatchResult{
		Status:     StatusRetryable,
		Retryable:  true,
		RetryAfter: retryAfter,
		Err:        err,
	}
}

func DispatchFailed(err error) DispatchResult {
	return DispatchResult{
		Status: StatusFailed,
		Err:    err,
	}
}
