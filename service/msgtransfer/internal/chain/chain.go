package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/model"
)

// Chain wires the Kafka write pipeline (03 §9 B1): the toTransfer hot path and
// the toPostgres persist consumer. It also produces msg.toPush.v1 — gateway
// fan-out itself now lives in service/push (03 §9 C2), so the chain no longer
// dispatches to the gateway.
type Chain struct {
	producer        *messaging.KafkaProducer
	transferConsume *messaging.KafkaConsumer
	persistConsume  *messaging.KafkaConsumer
	transferHandler *TransferHandler
	persistHandler  *PersistHandler
	rdb             *redis.Client
	brokers         []string

	wg sync.WaitGroup
}

// Options carries the pieces main already owns.
type Options struct {
	Kafka      config.TransferKafkaConfig
	DataSource string
}

func New(opts Options) (*Chain, error) {
	brokers := config.KafkaBrokerList(opts.Kafka.Brokers)
	if len(brokers) == 0 {
		return nil, errors.New("kafka chain enabled but no brokers configured")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     opts.Kafka.Redis.Addr,
		Password: opts.Kafka.Redis.Password,
		DB:       opts.Kafka.Redis.DB,
	})

	writer, err := model.NewWriter(opts.DataSource)
	if err != nil {
		return nil, err
	}
	seq, err := NewSeqAllocator(rdb, writer)
	if err != nil {
		return nil, err
	}
	store, err := NewStore(rdb)
	if err != nil {
		return nil, err
	}
	producer, err := messaging.NewKafkaProducer(brokers)
	if err != nil {
		return nil, err
	}
	transferHandler, err := NewTransferHandler(seq, store, producer, opts.Kafka.Workers, opts.Kafka.TypedAccountIDs)
	if err != nil {
		return nil, err
	}
	persistHandler, err := NewPersistHandler(writer)
	if err != nil {
		return nil, err
	}
	transferConsume, err := messaging.NewKafkaConsumer(brokers, messaging.GroupTransfer, []string{messaging.TopicToTransfer})
	if err != nil {
		return nil, err
	}
	persistConsume, err := messaging.NewKafkaConsumer(brokers, messaging.GroupPersist, []string{messaging.TopicToPostgres})
	if err != nil {
		return nil, err
	}

	return &Chain{
		producer:        producer,
		transferConsume: transferConsume,
		persistConsume:  persistConsume,
		transferHandler: transferHandler,
		persistHandler:  persistHandler,
		rdb:             rdb,
		brokers:         brokers,
	}, nil
}

// Start ensures topics then launches the two consumers. Non-blocking.
func (c *Chain) Start(ctx context.Context) error {
	ensureCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := messaging.EnsureTopics(ensureCtx, c.brokers,
		messaging.TopicToTransfer, messaging.TopicToPostgres, messaging.TopicToPush, messaging.TopicAgentTrigger); err != nil {
		return err
	}

	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		if err := c.transferConsume.Run(ctx, c.transferHandler.HandleBatch); err != nil && ctx.Err() == nil {
			logx.Errorf("msgtransfer kafka toTransfer consumer stopped: %v", err)
		}
	}()
	go func() {
		defer c.wg.Done()
		if err := c.persistConsume.Run(ctx, c.persistHandler.HandleBatch); err != nil && ctx.Err() == nil {
			logx.Errorf("msgtransfer kafka toPostgres consumer stopped: %v", err)
		}
	}()
	return nil
}

// Ready reports broker + redis connectivity for readyz.
func (c *Chain) Ready(ctx context.Context) error {
	if err := c.producer.Ping(ctx); err != nil {
		return fmt.Errorf("kafka: %w", err)
	}
	if err := c.rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

// Close stops consumers; safe after ctx cancellation.
func (c *Chain) Close() error {
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.transferConsume.Close()
	_ = c.persistConsume.Close()
	_ = c.producer.Close()
	done := make(chan struct{})
	go func() { c.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-stopCtx.Done():
	}
	return c.rdb.Close()
}
