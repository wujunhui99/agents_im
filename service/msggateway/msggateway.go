// msggateway（03 §9 A3，原 service/gateway-ws）：纯连接层——WebSocket 长连接 +
// JWT/session 鉴权 + presence + 下行推送 gRPC 面（GatewayService.BatchPushOneMsg，
// 03 §6.2，由 service/push 经 k8s headless DNS 广播）。4 个 ws command 经 msg-rpc
// gRPC 转发，不再装配 monolith repository / AI runtime。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/middleware"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/presence"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"github.com/wujunhui99/agents_im/service/msggateway/gatewaypb"
	"github.com/wujunhui99/agents_im/service/msggateway/internal/backend"
	"github.com/wujunhui99/agents_im/service/msggateway/internal/grpcserver"
	gatewayws "github.com/wujunhui99/agents_im/service/msggateway/internal/ws"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

func main() {
	configFile := flag.String("f", "etc/msggateway.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load msggateway config: %v", err)
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

	if !hasRPCClientConfig(cfg.MsgRPC) {
		log.Fatalf("msg rpc client config is required (MsgRPC.Target / MSG_RPC_TARGET)")
	}
	msgCli, err := zrpc.NewClient(cfg.MsgRPC)
	if err != nil {
		log.Fatalf("build msg rpc client: %v", err)
	}
	messageBackend := backend.NewMsgRPCBackend(msgclient.NewMsg(msgCli))

	presenceStore, err := presence.NewStore(cfg.Presence, cfg.Redis)
	if err != nil {
		log.Fatalf("build presence store: %v", err)
	}
	defer closePresenceStore(presenceStore)

	wsServer := gatewayws.NewServer(
		cfg.Auth,
		messageBackend,
		gatewayws.WithPresenceStore(presenceStore),
		gatewayws.WithPresenceTTL(presence.HeartbeatTTL(cfg.Presence)),
		gatewayws.WithInstanceID(gatewayInstanceID()),
		gatewayws.WithGatewayWSConfig(cfg.GatewayWS),
		gatewayws.WithSessionStore(middleware.NewRedisSessionStore(cfg.Redis)),
	)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsServer)
	mux.HandleFunc("/internal/delivery/conversation", wsServer.HandleInternalConversationDelivery)
	mux.HandleFunc("/internal/presence/user", wsServer.HandleInternalUserPresence)
	mux.HandleFunc("/healthz", health.LivenessHandler(cfg.Name))
	mux.HandleFunc("/readyz", health.ReadinessHandler(cfg.Name, func(*http.Request) []health.Check {
		return []health.Check{
			health.ComponentCheck("websocket_server", wsServer.Ready() == nil, readinessMessage(wsServer.Ready())),
			health.ComponentCheck("message_backend", messageBackend != nil, "configured"),
			health.ComponentCheck("presence_store", presenceStore != nil, "configured"),
		}
	}))
	mux.HandleFunc("/metrics", observability.MetricsHandler())

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           observability.TraceMiddleware(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("%s listening on %s", cfg.Name, server.Addr)
		errCh <- server.ListenAndServe()
	}()

	// 下行推送 gRPC server（03 §6.2）：service/push 经 k8s headless DNS 广播
	// BatchPushOneMsg，本实例只投递给自己持有的连接。空 ListenOn = 不启用。
	grpcServer, grpcErrCh := startGatewayGRPC(cfg.GatewayGRPC.ListenOn, wsServer)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		log.Printf("received %s, shutting down", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway server failed: %v", err)
		}
	case err := <-grpcErrCh:
		if err != nil {
			log.Fatalf("gateway push grpc server failed: %v", err)
		}
	}

	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown gateway server: %v", err)
	}
}

// startGatewayGRPC starts the downstream-push gRPC server when listenOn is set.
// Returns the server (nil when disabled) and a channel that receives a Serve error.
func startGatewayGRPC(listenOn string, deliverer grpcserver.PushDeliverer) (*grpc.Server, <-chan error) {
	errCh := make(chan error, 1)
	listenOn = strings.TrimSpace(listenOn)
	if listenOn == "" {
		log.Printf("gateway push grpc server disabled (GatewayGRPC.ListenOn empty)")
		return nil, errCh
	}
	listener, err := net.Listen("tcp", listenOn)
	if err != nil {
		log.Fatalf("listen gateway push grpc %s: %v", listenOn, err)
	}
	grpcServer := grpc.NewServer(grpc.ChainUnaryInterceptor(observability.GRPCUnaryServerInterceptor()))
	gatewaypb.RegisterGatewayServiceServer(grpcServer, grpcserver.New(deliverer))
	go func() {
		log.Printf("gateway push grpc server listening on %s", listenOn)
		errCh <- grpcServer.Serve(listener)
	}()
	return grpcServer, errCh
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}

func gatewayInstanceID() string {
	if value := strings.TrimSpace(os.Getenv("GATEWAY_INSTANCE_ID")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("AGENTS_IM_GATEWAY_INSTANCE_ID")); value != "" {
		return value
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		return "msggateway"
	}
	return strings.TrimSpace(hostname)
}

func closePresenceStore(store presence.PresenceStore) {
	closer, ok := store.(interface {
		Close() error
	})
	if !ok {
		return
	}
	if err := closer.Close(); err != nil {
		log.Printf("close presence store: %v", err)
	}
}

func readinessMessage(err error) string {
	if err != nil {
		return err.Error()
	}
	return "configured"
}
