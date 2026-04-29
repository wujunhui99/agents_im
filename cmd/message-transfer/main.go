package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/transfer"
)

func main() {
	configFile := flag.String("f", "etc/message-transfer.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadMessageTransferConfig(*configFile)
	if err != nil {
		log.Fatalf("load message transfer config: %v", err)
	}

	consumer := buildConsumer(cfg)
	dispatcher := buildDispatcher(cfg)
	worker := transfer.NewWorker(
		consumer,
		dispatcher,
		transfer.WithWorkerID(cfg.WorkerID),
		transfer.WithIdempotencyStore(transfer.NewMemoryIdempotencyStore()),
		transfer.WithPollInterval(time.Duration(cfg.Worker.PollIntervalMillis)*time.Millisecond),
		transfer.WithRetryBackoff(time.Duration(cfg.Worker.RetryBackoffMillis)*time.Millisecond),
		transfer.WithMaxAttempts(cfg.Worker.MaxAttempts),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf(
		"%s starting worker_id=%s topic=%s group=%s consumer=%s dispatcher=%s dry_run=%t",
		cfg.Name,
		cfg.WorkerID,
		cfg.Consumer.Topic,
		cfg.Consumer.Group,
		cfg.Consumer.Driver,
		cfg.Dispatcher.Driver,
		cfg.DryRun,
	)
	if err := worker.Run(ctx); err != nil {
		log.Fatalf("message transfer worker stopped with error: %v", err)
	}
	log.Printf("%s stopped", cfg.Name)
}

func buildConsumer(cfg config.MessageTransferConfig) transfer.EventConsumer {
	switch cfg.Consumer.Driver {
	default:
		return transfer.NewInMemoryConsumer()
	}
}

func buildDispatcher(cfg config.MessageTransferConfig) transfer.DeliveryDispatcher {
	switch cfg.Dispatcher.Driver {
	default:
		return transfer.NoopDispatcher{}
	}
}
