package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/wujunhui99/agents_im/internal/adminbootstrap"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
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
	agentRepo, err := userrepo.NewAgentRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent repository: %v", err)
	}
	agentRegistryRepo, err := userrepo.NewAgentRegistryRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent registry repository: %v", err)
	}
	mailer, err := mailadapter.NewRequiredRPCClient(cfg.MailRPC)
	if err != nil {
		log.Fatalf("build mail rpc client: %v", err)
	}
	userLogic := userlogic.NewUserLogic(userRepo)
	defaultAssistant := userlogic.NewDefaultAssistantProvisioner(userRepo, agentRepo, agentRegistryRepo)
	userLogic.WithDefaultAssistantProvisioner(defaultAssistant)
	if _, err := defaultAssistant.Backfill(context.Background()); err != nil {
		log.Fatalf("backfill default assistant: %v", err)
	}
	if created, err := adminbootstrap.EnsureAdminAccount(context.Background(), adminbootstrap.FromAPIConfig(cfg), userLogic, credentialRepo); err != nil {
		log.Fatalf("bootstrap admin account: %v", err)
	} else if created {
		log.Printf("admin bootstrap account ensured for identifier %q", cfg.AdminBootstrap.Identifier)
	}
	serviceContext := authsvc.NewServiceContextWithOptions(
		credentialRepo,
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(cfg.Auth.AccessSecret, time.Duration(cfg.Auth.AccessExpire)*time.Second),
		authlogic.AuthOptions{
			VerificationRepo: credentialRepo,
			Mailer:           mailer,
		},
	)
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	handler.RegisterAuthGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
