package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zeromicro/go-zero/core/logx"
)

// KafkaProducer is a thin franz-go wrapper with the pipeline's durability
// contract baked in: acks=all + idempotent producer (03 §3.2). franz-go enables
// both by default; they are restated here so a future option change cannot
// silently weaken the contract.
type KafkaProducer struct {
	client *kgo.Client
}

func NewKafkaProducer(brokers []string) (*KafkaProducer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are required")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.SnappyCompression(), kgo.NoCompression()),
	)
	if err != nil {
		return nil, fmt.Errorf("new kafka producer: %w", err)
	}
	return &KafkaProducer{client: client}, nil
}

// Publish produces one record synchronously and returns only after the broker
// acknowledged it (acks=all). Key is the partition key (conversation_id).
func (p *KafkaProducer) Publish(ctx context.Context, topic, key string, value []byte) error {
	if p == nil || p.client == nil {
		return errors.New("kafka producer is closed")
	}
	record := &kgo.Record{Topic: topic, Key: []byte(key), Value: value}
	return p.client.ProduceSync(ctx, record).FirstErr()
}

// PublishEvent marshals and publishes a MessageEvent keyed by conversation_id.
func (p *KafkaProducer) PublishEvent(ctx context.Context, topic string, event MessageEvent) error {
	payload, err := MarshalMessageEvent(event)
	if err != nil {
		return err
	}
	return p.Publish(ctx, topic, event.PartitionKey(), payload)
}

func (p *KafkaProducer) Ping(ctx context.Context) error {
	if p == nil || p.client == nil {
		return errors.New("kafka producer is closed")
	}
	return p.client.Ping(ctx)
}

func (p *KafkaProducer) Close() error {
	if p != nil && p.client != nil {
		p.client.Close()
	}
	return nil
}

// KafkaBatchHandler processes one polled batch. Returning nil commits the
// batch's offsets (at-least-once: redelivery after a crash is expected and must
// be converged by consumer-side idempotency).
type KafkaBatchHandler func(ctx context.Context, records []*kgo.Record) error

// KafkaConsumer is a consumer-group poll loop with manual commits: offsets are
// committed only after the handler finished the whole polled batch.
type KafkaConsumer struct {
	client  *kgo.Client
	group   string
	topics  []string
	backoff time.Duration
}

func NewKafkaConsumer(brokers []string, group string, topics []string) (*KafkaConsumer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka brokers are required")
	}
	if strings.TrimSpace(group) == "" {
		return nil, errors.New("kafka consumer group is required")
	}
	if len(topics) == 0 {
		return nil, errors.New("kafka consumer topics are required")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		// Without a committed offset start from the earliest record: the chain
		// must not silently skip messages produced before the first deploy.
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("new kafka consumer: %w", err)
	}
	return &KafkaConsumer{client: client, group: group, topics: topics, backoff: time.Second}, nil
}

// Run polls until ctx is cancelled. A handler error leaves offsets uncommitted
// and re-polls after a backoff — the same records are redelivered (fail-fast,
// no silent drop). Fatal client errors are returned.
func (c *KafkaConsumer) Run(ctx context.Context, handler KafkaBatchHandler) error {
	if c == nil || c.client == nil {
		return errors.New("kafka consumer is closed")
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		fetches := c.client.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return errors.New("kafka consumer client closed")
		}
		if err := ctx.Err(); err != nil {
			return nil
		}
		var fetchErr error
		fetches.EachError(func(topic string, partition int32, err error) {
			if !errors.Is(err, context.Canceled) {
				fetchErr = fmt.Errorf("kafka fetch %s/%d: %w", topic, partition, err)
			}
		})
		if fetchErr != nil {
			if !sleepCtx(ctx, c.backoff) {
				return nil
			}
			continue
		}
		records := fetches.Records()
		if len(records) == 0 {
			continue
		}
		// Retry the SAME batch in place until it succeeds: franz-go's poll
		// position advances in memory regardless of commits, so skipping ahead
		// after an error would silently drop the batch until a restart. Lag
		// growth + error logs are the intended failure signal (fail loudly).
		for attempt := 1; ; attempt++ {
			err := handler(ctx, records)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return nil
			}
			logBatchError(c.group, attempt, err)
			if !sleepCtx(ctx, c.backoff) {
				return nil
			}
		}
		for {
			err := c.client.CommitRecords(ctx, records...)
			if err == nil {
				break
			}
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return nil
			}
			// Keep retrying: proceeding uncommitted would re-process the batch
			// after a restart (safe) but masks a broker problem — surface it.
			logBatchError(c.group, 0, fmt.Errorf("commit offsets: %w", err))
			if !sleepCtx(ctx, c.backoff) {
				return nil
			}
		}
	}
}

func logBatchError(group string, attempt int, err error) {
	logx.Errorf("kafka consumer group=%s attempt=%d batch error: %v", group, attempt, err)
}

func (c *KafkaConsumer) Close() error {
	if c != nil && c.client != nil {
		c.client.Close()
	}
	return nil
}

// EnsureTopics idempotently creates the pipeline topics at boot. Partitions=1
// per topic: a single broker with conversation-keyed ordering does not benefit
// from more, and repartitioning later would transiently break per-conversation
// ordering — bump only together with a planned stop-the-world restart (03 §5.1).
func EnsureTopics(ctx context.Context, brokers []string, topics ...string) error {
	if len(brokers) == 0 {
		return errors.New("kafka brokers are required")
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}
	defer client.Close()
	admin := kadm.NewClient(client)
	resp, err := admin.CreateTopics(ctx, 1, 1, nil, topics...)
	if err != nil {
		return fmt.Errorf("ensure topics: %w", err)
	}
	for _, result := range resp.Sorted() {
		if result.Err != nil && !errors.Is(result.Err, kerr.TopicAlreadyExists) {
			return fmt.Errorf("ensure topic %s: %w", result.Topic, result.Err)
		}
	}
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
