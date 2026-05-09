package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/config"
	gatewayws "github.com/wujunhui99/agents_im/internal/gateway/ws"
	"github.com/wujunhui99/agents_im/internal/health"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/presence"
	"github.com/wujunhui99/agents_im/internal/repository"
	gatewaysvc "github.com/wujunhui99/agents_im/internal/servicecontext/gateway"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
)

func main() {
	configFile := flag.String("f", "etc/gateway-ws.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load gateway config: %v", err)
	}

	groupsRepo, err := repository.NewGroupsRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	presenceStore, err := presence.NewStore(cfg.Presence, cfg.Redis)
	if err != nil {
		log.Fatalf("build presence store: %v", err)
	}
	groupsLogic := logic.NewGroupsLogic(groupsRepo, nil)
	messageContext := messagesvc.NewServiceContextWithMedia(
		messageRepo,
		mediaRepo,
		nil,
		groupsLogic,
		cfg.Auth,
	)
	serviceContext := gatewaysvc.NewServiceContext(messageContext.MessageLogic, cfg.Auth)
	defer closePresenceStore(presenceStore)

	wsServer := gatewayws.NewServer(
		serviceContext,
		gatewayws.WithPresenceStore(presenceStore),
		gatewayws.WithPresenceTTL(presence.HeartbeatTTL(cfg.Presence)),
		gatewayws.WithInstanceID(gatewayInstanceID()),
		gatewayws.WithGatewayWSConfig(cfg.GatewayWS),
	)
	if config.ResolveStorageDriver(cfg.StorageDriver) == config.StorageDriverPostgres {
		authRepo, err := authrepo.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
		if err != nil {
			log.Fatalf("build auth repository: %v", err)
		}
		gatewayws.WithActiveSessionRepository(authRepo)(wsServer)
	} else {
		log.Printf("active session shared validation disabled for storage driver %q; use postgres for single-device enforcement across services", config.ResolveStorageDriver(cfg.StorageDriver))
	}

	mux := http.NewServeMux()
	mux.Handle("/ws", wsServer)
	mux.HandleFunc("/internal/delivery/conversation", wsServer.HandleInternalConversationDelivery)
	mux.HandleFunc("/internal/presence/user", wsServer.HandleInternalUserPresence)
	mux.HandleFunc("/healthz", health.LivenessHandler(cfg.Name))
	mux.HandleFunc("/readyz", health.ReadinessHandler(cfg.Name, func(*http.Request) []health.Check {
		return []health.Check{
			health.ComponentCheck("websocket_server", wsServer.Ready() == nil, readinessMessage(wsServer.Ready())),
			health.ComponentCheck("message_logic", serviceContext != nil && serviceContext.MessageLogic != nil, "configured"),
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

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		log.Printf("received %s, shutting down", sig)
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway server failed: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("shutdown gateway server: %v", err)
	}
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
		return "gateway-ws"
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
