// push（03-message-pipeline §6 / §9 C2-C3，对齐 open-im-server internal/push）：消息
// 投递调度器,不做持久化。消费 msg.toPush.v1 → 经 gRPC 广播到所有 msggateway 实例
// (k8s headless DNS,无 etcd) → 在线投递失败/离线的 user 二段式 produce msg.toOfflinePush.v1
// → 自身离线 consumer 调厂商通道(首版 audit-only)。
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

	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
	pushgateway "github.com/wujunhui99/agents_im/service/push/internal/gateway"
	"github.com/wujunhui99/agents_im/service/push/internal/pusher"
	"github.com/zeromicro/go-zero/core/logx"
)

func main() {
	configFile := flag.String("f", "etc/push.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadPushConfig(*configFile)
	if err != nil {
		log.Fatalf("load push config: %v", err)
	}
	brokers := config.KafkaBrokerList(cfg.Kafka.Brokers)
	if len(brokers) == 0 {
		log.Fatalf("push requires Kafka brokers (Kafka.Brokers / KAFKA_BROKERS)")
	}
	if cfg.Gateway.Target == "" {
		log.Fatalf("push requires a gateway target (Gateway.Target / PUSH_GATEWAY_TARGET)")
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	broadcaster, err := pushgateway.NewBroadcaster(
		cfg.Gateway.Target,
		time.Duration(cfg.Gateway.DialTimeoutSeconds)*time.Second,
		time.Duration(cfg.Gateway.PushTimeoutSeconds)*time.Second,
		time.Duration(cfg.Gateway.RefreshSeconds)*time.Second,
	)
	if err != nil {
		log.Fatalf("build gateway broadcaster: %v", err)
	}
	if err := broadcaster.Start(ctx); err != nil {
		log.Fatalf("start gateway broadcaster: %v", err)
	}
	defer broadcaster.Close()

	producer, err := messaging.NewKafkaProducer(brokers)
	if err != nil {
		log.Fatalf("build kafka producer: %v", err)
	}
	defer producer.Close()

	ensureCtx, cancelEnsure := context.WithTimeout(ctx, 30*time.Second)
	if err := messaging.EnsureTopics(ensureCtx, brokers, messaging.TopicToPush, messaging.TopicToOfflinePush); err != nil {
		cancelEnsure()
		log.Fatalf("ensure kafka topics: %v", err)
	}
	cancelEnsure()

	onlineHandler, err := pusher.NewOnlineHandler(broadcaster, producer)
	if err != nil {
		log.Fatalf("build online handler: %v", err)
	}
	offlineHandler, err := pusher.NewOfflineHandler(pusher.NewAuditOfflinePusher())
	if err != nil {
		log.Fatalf("build offline handler: %v", err)
	}

	onlineConsumer, err := messaging.NewKafkaConsumer(brokers, messaging.GroupPushOnline, []string{messaging.TopicToPush})
	if err != nil {
		log.Fatalf("build online consumer: %v", err)
	}
	defer onlineConsumer.Close()
	offlineConsumer, err := messaging.NewKafkaConsumer(brokers, messaging.GroupPushOffline, []string{messaging.TopicToOfflinePush})
	if err != nil {
		log.Fatalf("build offline consumer: %v", err)
	}
	defer offlineConsumer.Close()

	go func() {
		if err := onlineConsumer.Run(ctx, onlineHandler.HandleBatch); err != nil && ctx.Err() == nil {
			logx.Errorf("push online consumer stopped: %v", err)
		}
	}()
	go func() {
		if err := offlineConsumer.Run(ctx, offlineHandler.HandleBatch); err != nil && ctx.Err() == nil {
			logx.Errorf("push offline consumer stopped: %v", err)
		}
	}()

	observabilityServer := startObservabilityHTTP(ctx, cfg, broadcaster, producer)
	defer shutdownObservabilityHTTP(observabilityServer)

	log.Printf("%s started brokers=%s gateway=%s topics=%s,%s", cfg.Name, cfg.Kafka.Brokers, cfg.Gateway.Target,
		messaging.TopicToPush, messaging.TopicToOfflinePush)
	<-ctx.Done()
	log.Printf("%s stopped", cfg.Name)
}

func startObservabilityHTTP(ctx context.Context, cfg config.PushConfig, broadcaster *pushgateway.Broadcaster, producer *messaging.KafkaProducer) *http.Server {
	if !cfg.Observability.Enabled {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health.LivenessHandler(cfg.Name))
	mux.HandleFunc("/readyz", health.ReadinessHandler(cfg.Name, func(r *http.Request) []health.Check {
		gatewayErr := broadcaster.Ready(r.Context())
		producerErr := producer.Ping(r.Context())
		return []health.Check{
			health.ComponentCheck("gateway_conns", gatewayErr == nil, readinessMessage(gatewayErr)),
			health.ComponentCheck("kafka_producer", producerErr == nil, readinessMessage(producerErr)),
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
			log.Printf("push observability server stopped with error: %v", err)
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
		log.Printf("shutdown push observability server: %v", err)
	}
}

func readinessMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return "ok"
}
