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

	"github.com/wujunhui99/agents_im/internal/config"
	gatewayws "github.com/wujunhui99/agents_im/internal/gateway/ws"
	"github.com/wujunhui99/agents_im/internal/presence"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func main() {
	configFile := flag.String("f", "etc/gateway-ws.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load gateway config: %v", err)
	}

	serviceContext := svc.NewMessageServiceContextWithAuth(
		repository.MustMessageRepositoryForStorage(cfg.StorageDriver, cfg.DataSource),
		nil,
		nil,
		cfg.Auth,
	)
	presenceStore := presence.MustStore(cfg.Presence, cfg.Redis)
	defer closePresenceStore(presenceStore)

	wsServer := gatewayws.NewServer(
		serviceContext,
		gatewayws.WithPresenceStore(presenceStore),
		gatewayws.WithPresenceTTL(presence.HeartbeatTTL(cfg.Presence)),
		gatewayws.WithInstanceID(gatewayInstanceID()),
	)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsServer)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:           mux,
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
