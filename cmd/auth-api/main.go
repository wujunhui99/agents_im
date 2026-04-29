package main

import (
	"flag"
	"log"
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

	userLogic := userlogic.NewUserLogic(userrepo.NewMemoryRepository())
	serviceContext := svc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(cfg.Auth.AccessSecret, time.Duration(cfg.Auth.AccessExpire)*time.Second),
	)
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg))
	defer server.Stop()
	handler.RegisterGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
