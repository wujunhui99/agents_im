package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/transfer"
)

func main() {
	configFile := flag.String("f", "etc/message-transfer.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadMessageTransferConfig(*configFile)
	if err != nil {
		log.Fatalf("load message transfer config: %v", err)
	}

	consumer, err := buildConsumer(cfg)
	if err != nil {
		log.Fatalf("build message transfer consumer: %v", err)
	}
	if closer, ok := consumer.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Printf("close message transfer consumer: %v", err)
			}
		}()
	}
	dispatcher := buildDispatcher(cfg)
	recorder, err := buildDeliveryAttemptRecorder(cfg)
	if err != nil {
		log.Fatalf("build delivery attempt recorder: %v", err)
	}
	workerOptions := []transfer.WorkerOption{
		transfer.WithWorkerID(cfg.WorkerID),
		transfer.WithIdempotencyStore(transfer.NewMemoryIdempotencyStore()),
		transfer.WithPollInterval(time.Duration(cfg.Worker.PollIntervalMillis) * time.Millisecond),
		transfer.WithRetryBackoff(time.Duration(cfg.Worker.RetryBackoffMillis) * time.Millisecond),
		transfer.WithMaxAttempts(cfg.Worker.MaxAttempts),
	}
	if recorder != nil {
		workerOptions = append(workerOptions, transfer.WithDeliveryAttemptRecorder(recorder))
	}
	worker := transfer.NewWorker(consumer, dispatcher, workerOptions...)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf(
		"%s starting worker_id=%s topic=%s group=%s consumer=%s dispatcher=%s storage=%s dry_run=%t",
		cfg.Name,
		cfg.WorkerID,
		cfg.Consumer.Topic,
		cfg.Consumer.Group,
		cfg.Consumer.Driver,
		cfg.Dispatcher.Driver,
		cfg.StorageDriver,
		cfg.DryRun,
	)
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("message transfer worker stopped with error: %v", err)
	}
	log.Printf("%s stopped", cfg.Name)
}

func buildConsumer(cfg config.MessageTransferConfig) (transfer.EventConsumer, error) {
	switch cfg.Consumer.Driver {
	case config.TransferConsumerKafka:
		return transfer.NewKafkaEventConsumer(transfer.KafkaEventConsumerConfig{
			Brokers: cfg.Kafka.Brokers,
			Topic:   cfg.Consumer.Topic,
			GroupID: cfg.Consumer.Group,
		})
	default:
		return transfer.NewInMemoryConsumer(), nil
	}
}

func buildDispatcher(cfg config.MessageTransferConfig) transfer.DeliveryDispatcher {
	switch cfg.Dispatcher.Driver {
	default:
		return transfer.NoopDispatcher{}
	}
}

func buildDeliveryAttemptRecorder(cfg config.MessageTransferConfig) (transfer.DeliveryAttemptRecorder, error) {
	switch cfg.StorageDriver {
	case config.StorageDriverPostgres:
		repo, err := repository.NewPostgresMessageRepository(cfg.DataSource)
		if err != nil {
			return nil, err
		}
		return transfer.NewRepositoryDeliveryAttemptRecorder(repo), nil
	default:
		return transfer.NewRepositoryDeliveryAttemptRecorder(repository.NewMemoryMessageRepository()), nil
	}
}
