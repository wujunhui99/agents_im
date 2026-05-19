package main

import (
	"flag"
	"log"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	adminsvc "github.com/wujunhui99/agents_im/internal/servicecontext/admin"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/message-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	groupsRepo, err := repository.NewGroupsRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	accountRepo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	agentHostingRepo, err := repository.NewAgentConversationHostingRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent hosting repository: %v", err)
	}
	aiHostingRepo, err := repository.NewConversationAIHostingRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build AI hosting repository: %v", err)
	}
	agentAuditRepo, err := repository.NewAgentAuditRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent audit repository: %v", err)
	}
	groupsLogic := logic.NewGroupsLogic(groupsRepo, nil)
	serviceContext := messagesvc.NewServiceContextWithMedia(
		messageRepo,
		mediaRepo,
		nil,
		groupsLogic,
		cfg.Auth,
	)
	serviceContext.AgentHostingRepo = agentHostingRepo
	serviceContext.AIHostingRepo = aiHostingRepo
	serviceContext.AIHostingLogic = logic.NewConversationAIHostingLogic(aiHostingRepo)
	serviceContext.AgentAuditRepo = agentAuditRepo
	serviceContext.AgentAuditLogic = logic.NewAgentAuditLogic(agentAuditRepo)
	if err := messagesvc.ConfigureConversationAIHosting(serviceContext, cfg.DeepSeek, cfg.LLMObservability); err != nil {
		log.Fatalf("configure AI conversation hosting: %v", err)
	}
	var adminContext *adminsvc.ServiceContext
	if config.ResolveStorageDriver(cfg.StorageDriver) == config.StorageDriverPostgres {
		postgresAccountRepo, ok := accountRepo.(*repository.PostgresRepository)
		if !ok {
			log.Fatalf("postgres account repository has unexpected type %T", accountRepo)
		}
		postgresMessageRepo, ok := messageRepo.(*repository.PostgresMessageRepository)
		if !ok {
			log.Fatalf("postgres message repository has unexpected type %T", messageRepo)
		}
		postgresAgentAuditRepo, ok := agentAuditRepo.(*repository.PostgresAgentAuditRepository)
		if !ok {
			log.Fatalf("postgres agent audit repository has unexpected type %T", agentAuditRepo)
		}
		adminContext = adminsvc.NewServiceContextWithAuth(adminsvc.Dependencies{
			Accounts:    postgresAccountRepo,
			Friends:     postgresAccountRepo,
			Messages:    postgresMessageRepo,
			AgentAudits: postgresAgentAuditRepo,
		}, cfg.Auth)
	} else {
		memoryAccountRepo, ok := accountRepo.(*repository.MemoryRepository)
		if !ok {
			log.Fatalf("memory account repository has unexpected type %T", accountRepo)
		}
		memoryMessageRepo, ok := messageRepo.(*repository.MemoryMessageRepository)
		if !ok {
			log.Fatalf("memory message repository has unexpected type %T", messageRepo)
		}
		memoryAgentAuditRepo, ok := agentAuditRepo.(*repository.MemoryAgentAuditRepository)
		if !ok {
			log.Fatalf("memory agent audit repository has unexpected type %T", agentAuditRepo)
		}
		adminContext = adminsvc.NewServiceContextWithAuth(adminsvc.Dependencies{
			Accounts:    memoryAccountRepo,
			Friends:     memoryAccountRepo,
			Messages:    memoryMessageRepo,
			AgentAudits: memoryAgentAuditRepo,
		}, cfg.Auth)
	}
	if config.ResolveStorageDriver(cfg.StorageDriver) == config.StorageDriverPostgres {
		authRepo, err := authrepo.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
		if err != nil {
			log.Fatalf("build auth repository: %v", err)
		}
		serviceContext.AuthSessions = authRepo
		adminContext.AuthSessions = authRepo
	} else {
		log.Printf("active session shared validation disabled for storage driver %q; use postgres for single-device enforcement across services", config.ResolveStorageDriver(cfg.StorageDriver))
	}
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg), rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	handler.RegisterMessageGoZeroHandlers(server, serviceContext)
	handler.RegisterAdminGoZeroHandlers(server, adminContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
