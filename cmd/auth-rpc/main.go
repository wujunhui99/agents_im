package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	authrpc "github.com/wujunhui99/agents_im/internal/auth/rpc"
	"github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/config"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
)

func main() {
	configFile := flag.String("f", "etc/auth-rpc.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadRPCConfig(*configFile)
	if err != nil {
		log.Fatalf("load rpc config: %v", err)
	}

	userLogic := userlogic.NewUserLogic(userrepo.NewMemoryRepository())
	serviceContext := svc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(tokenSecret(), tokenTTL()),
	)
	_ = authrpc.NewAuthServer(serviceContext.AuthLogic)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"code":"OK","message":"ok","data":{"status":"auth rpc contract initialized"}}` + "\n"))
	})

	log.Printf("%s contract initialized on %s; gRPC transport should be generated with goctl/protoc when available", cfg.Name, cfg.ListenOn)
	if err := http.ListenAndServe(cfg.ListenOn, mux); err != nil {
		log.Fatal(err)
	}
}

func tokenSecret() string {
	if value := os.Getenv("AUTH_TOKEN_SECRET"); value != "" {
		return value
	}
	return "dev-auth-secret-change-me"
}

func tokenTTL() time.Duration {
	value := os.Getenv("AUTH_TOKEN_TTL")
	if value == "" {
		return 24 * time.Hour
	}

	ttl, err := time.ParseDuration(value)
	if err == nil {
		return ttl
	}

	seconds, err := strconv.Atoi(value)
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	return 24 * time.Hour
}
