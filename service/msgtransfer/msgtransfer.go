package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/chain"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/transfer"
)

func readinessMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return "ok"
}

func main() {
	configFile := flag.String("f", "etc/msgtransfer.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadMessageTransferConfig(*configFile)
	if err != nil {
		log.Fatalf("load message transfer config: %v", err)
	}
	shutdownTracing, err := observability.InitServiceTracing(context.Background(), cfg.Tracing, cfg.Name)
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() {
		if err := observability.ShutdownTracing(shutdownTracing); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()

	dispatcher := buildDispatcher(cfg)
	recorder, err := buildDeliveryAttemptRecorder(cfg)
	if err != nil {
		log.Fatalf("build delivery attempt recorder: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Kafka 链路是唯一消费路径（03 §9 B3b）：legacy outbox worker 已退役，
	// 缺 Kafka 配置直接启动失败（失败优先），不再静默空转。
	kafkaChain, err := chain.New(chain.Options{
		Kafka:      cfg.Kafka,
		DataSource: cfg.DataSource,
		Dispatcher: dispatcher,
		Recorder:   recorder,
		WorkerID:   cfg.WorkerID,
	})
	if err != nil {
		log.Fatalf("build kafka chain: %v", err)
	}
	if err := kafkaChain.Start(ctx); err != nil {
		log.Fatalf("start kafka chain: %v", err)
	}
	defer func() {
		if err := kafkaChain.Close(); err != nil {
			log.Printf("close kafka chain: %v", err)
		}
	}()
	log.Printf("%s kafka chain started brokers=%s topics=%s,%s,%s,%s", cfg.Name, cfg.Kafka.Brokers,
		messaging.TopicToTransfer, messaging.TopicToPostgres, messaging.TopicToPush, messaging.TopicAgentTrigger)

	observabilityServer := startObservabilityHTTP(ctx, cfg, dispatcher, kafkaChain)
	defer shutdownObservabilityHTTP(observabilityServer)

	log.Printf(
		"%s starting worker_id=%s dispatcher=%s storage=%s dry_run=%t",
		cfg.Name,
		cfg.WorkerID,
		cfg.Dispatcher.Driver,
		cfg.StorageDriver,
		cfg.DryRun,
	)
	<-ctx.Done()
	log.Printf("%s stopped", cfg.Name)
}

func buildDispatcher(cfg config.MessageTransferConfig) transfer.DeliveryDispatcher {
	switch cfg.Dispatcher.Driver {
	case config.TransferDispatcherGateway:
		return transfer.NewGatewayHTTPDispatcher(transfer.GatewayHTTPDispatcherConfig{
			Endpoint: cfg.Dispatcher.GatewayEndpoint,
		})
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

func startObservabilityHTTP(ctx context.Context, cfg config.MessageTransferConfig, dispatcher transfer.DeliveryDispatcher, kafkaChain *chain.Chain) *http.Server {
	if !cfg.Observability.Enabled {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.LivenessHandler(cfg.Name))
	mux.HandleFunc("/readyz", health.ReadinessHandler(cfg.Name, func(r *http.Request) []health.Check {
		err := kafkaChain.Ready(r.Context())
		return []health.Check{
			health.ComponentCheck("delivery_dispatcher", dispatcher != nil, "configured"),
			health.ComponentCheck("kafka_chain", err == nil, readinessMessage(err)),
		}
	}))
	mux.HandleFunc("/metrics", observability.MetricsHandler())

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Observability.Host, cfg.Observability.Port),
		Handler:           observability.TraceMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("%s observability listening on %s", cfg.Name, server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("message transfer observability server stopped with error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownObservabilityHTTP(server)
	}()
	return server
}

func shutdownObservabilityHTTP(server *http.Server) {
	if server == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
		log.Printf("shutdown message transfer observability server: %v", err)
	}
}
