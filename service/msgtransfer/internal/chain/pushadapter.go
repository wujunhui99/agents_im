package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/internal/transfer"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
)

// KafkaPushConsumer adapts msg.toPush.v1 to the existing transfer.Worker
// (transfer.EventConsumer), so gateway dispatch keeps its retry/idempotency/
// delivery-attempt semantics until 03 §9 C2 splits service/push.
//
// Offset contract: a record's offset is committed on MarkSuccessful/MarkFailed.
// MarkRetry requeues in memory WITHOUT committing — a crash mid-retry usually
// redelivers from Kafka (unless a later offset on the same partition was already
// committed, which implicitly commits the earlier one; push is best-effort beyond
// that — the message is durably in PG and clients converge by pulling, 03 §8.3).
// It is a keystone exception that this package imports internal/transfer; both
// go away together (B3/C2).
type KafkaPushConsumer struct {
	client  *kgo.Client
	pending chan pushEnvelope

	mu      sync.Mutex
	retries []pushRetry
	started bool
}

type pushEnvelope struct {
	envelope transfer.Envelope
	record   *kgo.Record
}

type pushRetry struct {
	item    pushEnvelope
	readyAt time.Time
}

func NewKafkaPushConsumer(brokers []string) (*KafkaPushConsumer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka push consumer requires brokers")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(messaging.GroupPush),
		kgo.ConsumeTopics(messaging.TopicToPush),
		kgo.DisableAutoCommit(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("new kafka push consumer: %w", err)
	}
	return &KafkaPushConsumer{
		client:  client,
		pending: make(chan pushEnvelope, 256),
	}, nil
}

// Start launches the poll loop feeding the worker. Blocks until ctx ends.
func (c *KafkaPushConsumer) Start(ctx context.Context) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return
	}
	c.started = true
	c.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return
		}
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() || ctx.Err() != nil {
			return
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			if !errors.Is(err, context.Canceled) {
				logx.WithContext(ctx).Errorf("msgtransfer push consumer fetch %s/%d: %v", topic, partition, err)
			}
		})
		for _, record := range fetches.Records() {
			item, err := pushEnvelopeFromRecord(record)
			if err != nil {
				logx.WithContext(ctx).Errorf("msgtransfer: drop malformed toPush record offset=%d: %v", record.Offset, err)
				c.commit(ctx, record)
				continue
			}
			select {
			case c.pending <- item:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Receive implements transfer.EventConsumer.
func (c *KafkaPushConsumer) Receive(ctx context.Context) (transfer.Envelope, error) {
	if item, ok := c.popReadyRetry(); ok {
		return item.envelope, nil
	}
	select {
	case item := <-c.pending:
		c.track(item)
		return item.envelope, nil
	default:
		return transfer.Envelope{}, transfer.ErrNoEvent
	}
}

func (c *KafkaPushConsumer) MarkSuccessful(ctx context.Context, envelope transfer.Envelope) error {
	c.commitTracked(ctx, envelope.ID)
	return nil
}

func (c *KafkaPushConsumer) MarkRetry(ctx context.Context, envelope transfer.Envelope, decision transfer.RetryDecision) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.retries {
		if c.retries[i].item.envelope.ID == envelope.ID {
			c.retries[i].item.envelope.Attempt = decision.Attempt + 1
			c.retries[i].readyAt = decision.NextAttemptAt
			return nil
		}
	}
	return fmt.Errorf("kafka push consumer: retry for unknown envelope %s", envelope.ID)
}

func (c *KafkaPushConsumer) MarkFailed(ctx context.Context, envelope transfer.Envelope, result transfer.ProcessResult) error {
	c.commitTracked(ctx, envelope.ID)
	return nil
}

func (c *KafkaPushConsumer) Close() error {
	if c != nil && c.client != nil {
		c.client.Close()
	}
	return nil
}

// track registers an in-flight record so MarkRetry can find it again.
func (c *KafkaPushConsumer) track(item pushEnvelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.retries = append(c.retries, pushRetry{item: item, readyAt: time.Time{}})
}

func (c *KafkaPushConsumer) popReadyRetry() (pushEnvelope, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for i := range c.retries {
		retry := c.retries[i]
		if !retry.readyAt.IsZero() && !retry.readyAt.After(now) {
			c.retries[i].readyAt = time.Time{} // back in flight
			return retry.item, true
		}
	}
	return pushEnvelope{}, false
}

func (c *KafkaPushConsumer) commitTracked(ctx context.Context, envelopeID string) {
	c.mu.Lock()
	var record *kgo.Record
	for i := range c.retries {
		if c.retries[i].item.envelope.ID == envelopeID {
			record = c.retries[i].item.record
			c.retries = append(c.retries[:i], c.retries[i+1:]...)
			break
		}
	}
	c.mu.Unlock()
	if record != nil {
		c.commit(ctx, record)
	}
}

func (c *KafkaPushConsumer) commit(ctx context.Context, record *kgo.Record) {
	if err := c.client.CommitRecords(context.WithoutCancel(ctx), record); err != nil {
		logx.WithContext(ctx).Errorf("msgtransfer push consumer commit offset=%d: %v", record.Offset, err)
	}
}

func pushEnvelopeFromRecord(record *kgo.Record) (pushEnvelope, error) {
	event, err := messaging.UnmarshalMessageEvent(record.Value)
	if err != nil {
		return pushEnvelope{}, err
	}
	if event.EventType != messaging.EventTypeMessageAccepted {
		return pushEnvelope{}, fmt.Errorf("unexpected toPush event_type %q", event.EventType)
	}
	envelope := transfer.Envelope{
		ID:         event.EventID,
		Topic:      record.Topic,
		Key:        event.PartitionKey(),
		Partition:  record.Partition,
		Offset:     record.Offset,
		Attempt:    1,
		ReceivedAt: time.Now().UTC(),
		Event:      transfer.MessageEventFromMessagingEvent(event),
		TraceContext: observability.TraceContext{
			TraceID:     event.Payload.TraceID,
			RequestID:   event.Payload.RequestID,
			TraceParent: event.Payload.TraceParent,
			TraceState:  event.Payload.TraceState,
		},
		RawPayload: append([]byte(nil), record.Value...),
	}
	return pushEnvelope{envelope: envelope, record: record}, nil
}
