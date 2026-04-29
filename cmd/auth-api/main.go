package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/handler"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/config"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
)

func main() {
	configFile := flag.String("f", "etc/auth-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	userLogic := userlogic.NewUserLogic(userrepo.NewMemoryRepository())
	serviceContext := svc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(tokenSecret(), tokenTTL()),
	)

	mux := http.NewServeMux()
	handler.RegisterHandlers(mux, serviceContext)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("%s listening on %s", cfg.Name, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
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
