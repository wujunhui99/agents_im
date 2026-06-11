package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/internal/transfer"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/model"
)

// Chain wires the Kafka write pipeline (03 §9 B1): toTransfer hot path,
// toPostgres persist consumer, and toPush → gateway dispatch via the existing
// transfer.Worker. Runs alongside the legacy outbox worker until B3.
type Chain struct {
	producer        *messaging.KafkaProducer
	transferConsume *messaging.KafkaConsumer
	persistConsume  *messaging.KafkaConsumer
	pushConsumer    *KafkaPushConsumer
	pushWorker      *transfer.Worker
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
	Dispatcher transfer.DeliveryDispatcher
	Recorder   transfer.DeliveryAttemptRecorder
	WorkerID   string
}

func New(opts Options) (*Chain, error) {
	brokers := config.KafkaBrokerList(opts.Kafka.Brokers)
	if len(brokers) == 0 {
		return nil, errors.New("kafka chain enabled but no brokers configured")
	}
	if opts.Dispatcher == nil {
		return nil, errors.New("kafka chain requires a delivery dispatcher")
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
	transferHandler, err := NewTransferHandler(seq, store, producer, opts.Kafka.Workers)
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
	pushConsumer, err := NewKafkaPushConsumer(brokers)
	if err != nil {
		return nil, err
	}
	workerOptions := []transfer.WorkerOption{
		transfer.WithWorkerID(opts.WorkerID + "-kafka-push"),
		transfer.WithIdempotencyStore(transfer.NewMemoryIdempotencyStore()),
	}
	if opts.Recorder != nil {
		workerOptions = append(workerOptions, transfer.WithDeliveryAttemptRecorder(opts.Recorder))
	}
	pushWorker := transfer.NewWorker(pushConsumer, opts.Dispatcher, workerOptions...)

	return &Chain{
		producer:        producer,
		transferConsume: transferConsume,
		persistConsume:  persistConsume,
		pushConsumer:    pushConsumer,
		pushWorker:      pushWorker,
		transferHandler: transferHandler,
		persistHandler:  persistHandler,
		rdb:             rdb,
		brokers:         brokers,
	}, nil
}

// Start ensures topics then launches the three consumers. Non-blocking.
func (c *Chain) Start(ctx context.Context) error {
	ensureCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := messaging.EnsureTopics(ensureCtx, c.brokers,
		messaging.TopicToTransfer, messaging.TopicToPostgres, messaging.TopicToPush, messaging.TopicAgentTrigger); err != nil {
		return err
	}

	c.wg.Add(3)
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
	go func() {
		defer c.wg.Done()
		c.pushConsumer.Start(ctx)
	}()
	if err := c.pushWorker.Start(ctx); err != nil {
		return fmt.Errorf("start kafka push worker: %w", err)
	}
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
	_ = c.pushWorker.Stop(stopCtx)
	_ = c.transferConsume.Close()
	_ = c.persistConsume.Close()
	_ = c.pushConsumer.Close()
	_ = c.producer.Close()
	done := make(chan struct{})
	go func() { c.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-stopCtx.Done():
	}
	return c.rdb.Close()
}
