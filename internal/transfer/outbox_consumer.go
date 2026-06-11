package transfer

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/outboxpublisher"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/messaging"
)

const (
	defaultOutboxConsumerBatchLimit   = 100
	defaultOutboxConsumerLockDuration = 30 * time.Second
)

type OutboxEventConsumerConfig struct {
	Repository   repository.OutboxRepository
	WorkerID     string
	BatchLimit   int
	LockDuration time.Duration
	Now          func() time.Time
}

type OutboxEventConsumer struct {
	repo         repository.OutboxRepository
	workerID     string
	batchLimit   int
	lockDuration time.Duration
	now          func() time.Time
	pending      []repository.OutboxEvent
}

func NewOutboxEventConsumer(cfg OutboxEventConsumerConfig) (*OutboxEventConsumer, error) {
	if cfg.Repository == nil {
		return nil, errors.New("outbox repository is required")
	}
	workerID := strings.TrimSpace(cfg.WorkerID)
	if workerID == "" {
		workerID = "message-transfer-outbox"
	}
	batchLimit := cfg.BatchLimit
	if batchLimit <= 0 {
		batchLimit = defaultOutboxConsumerBatchLimit
	}
	lockDuration := cfg.LockDuration
	if lockDuration <= 0 {
		lockDuration = defaultOutboxConsumerLockDuration
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &OutboxEventConsumer{
		repo:         cfg.Repository,
		workerID:     workerID,
		batchLimit:   batchLimit,
		lockDuration: lockDuration,
		now:          now,
	}, nil
}

func (c *OutboxEventConsumer) Receive(ctx context.Context) (Envelope, error) {
	if err := ctx.Err(); err != nil {
		return Envelope{}, err
	}
	if c == nil || c.repo == nil {
		return Envelope{}, errors.New("outbox event consumer is closed")
	}

	for {
		if len(c.pending) == 0 {
			events, err := c.repo.PollPending(ctx, c.workerID, c.batchLimit, c.lockDuration)
			if err != nil {
				return Envelope{}, err
			}
			if len(events) == 0 {
				return Envelope{}, ErrNoEvent
			}
			c.pending = events
		}

		event := c.pending[0]
		c.pending = c.pending[1:]
		envelope, err := EnvelopeFromOutboxEvent(event)
		if err == nil {
			return envelope, nil
		}
		_ = c.repo.MarkFailed(ctx, event.EventID, c.workerID, repository.OutboxFailure{
			LastError: err.Error(),
		})
	}
}

func (c *OutboxEventConsumer) MarkSuccessful(ctx context.Context, envelope Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.repo == nil {
		return errors.New("outbox event consumer is closed")
	}
	return c.repo.MarkPublished(ctx, envelope.ID, c.workerID)
}

func (c *OutboxEventConsumer) MarkRetry(ctx context.Context, envelope Envelope, decision RetryDecision) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.repo == nil {
		return errors.New("outbox event consumer is closed")
	}
	return c.repo.MarkFailed(ctx, envelope.ID, c.workerID, repository.OutboxFailure{
		NextAttemptAt: decision.NextAttemptAt,
		LastError:     decision.Reason,
	})
}

func (c *OutboxEventConsumer) MarkFailed(ctx context.Context, envelope Envelope, result ProcessResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c == nil || c.repo == nil {
		return errors.New("outbox event consumer is closed")
	}
	return c.repo.MarkFailed(ctx, envelope.ID, c.workerID, repository.OutboxFailure{
		LastError: processResultError(result),
	})
}

func EnvelopeFromOutboxEvent(event repository.OutboxEvent) (Envelope, error) {
	messageEvent, err := outboxpublisher.MessageEventFromOutbox(event)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{
		ID:           event.EventID,
		Topic:        messaging.DefaultMessageEventsTopic,
		Key:          messageEvent.PartitionKey(),
		Attempt:      event.AttemptCount,
		ReceivedAt:   time.Now().UTC(),
		Event:        MessageEventFromMessagingEvent(messageEvent),
		TraceContext: traceContextFromMessagingEvent(messageEvent),
		RawPayload:   append([]byte(nil), event.Payload...),
	}, nil
}

func processResultError(result ProcessResult) string {
	if result.Err == nil {
		return ""
	}
	return strings.TrimSpace(result.Err.Error())
}
