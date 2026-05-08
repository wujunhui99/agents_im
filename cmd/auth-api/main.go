package main

import (
	"flag"
	"log"
	"time"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/observability"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
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

	userRepo, err := userrepo.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	credentialRepo, err := authrepo.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build auth repository: %v", err)
	}
	userLogic := userlogic.NewUserLogic(userRepo)
	serviceContext := authsvc.NewServiceContext(
		credentialRepo,
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(cfg.Auth.AccessSecret, time.Duration(cfg.Auth.AccessExpire)*time.Second),
	)
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	handler.RegisterAuthGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
