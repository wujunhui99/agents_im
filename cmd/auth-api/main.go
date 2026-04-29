package main

import (
	"flag"
	"log"
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
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/auth-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	userLogic := userlogic.NewUserLogic(userrepo.MustRepositoryForStorage(cfg.StorageDriver, cfg.DataSource))
	serviceContext := svc.NewServiceContext(
		authrepo.MustRepositoryForStorage(cfg.StorageDriver, cfg.DataSource),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(tokenSecret(), tokenTTL()),
	)
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg))
	defer server.Stop()
	handler.RegisterGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
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
