package main

import (
	"context"
	"flag"
	"log"

	"github.com/wujunhui99/agents_im/internal/agent/pythonexec"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/agent-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
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

	userRepo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	agentRepo, err := repository.NewAgentRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent repository: %v", err)
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent registry repository: %v", err)
	}
	userLogic := logic.NewUserLogic(userRepo)
	var pythonExecutorClient pythonexec.KubernetesSandboxClient
	if cfg.PythonExecutor.Backend == config.PythonExecutorBackendK8S {
		pythonExecutorClient, err = pythonexec.NewInClusterKubernetesSandboxClient()
		if err != nil {
			log.Fatalf("build python executor kubernetes client: %v", err)
		}
	}
	pythonExecutor, err := pythonexec.NewExecutorFromConfig(cfg.PythonExecutor, pythonExecutorClient)
	if err != nil {
		log.Fatalf("build python executor: %v", err)
	}
	serviceContext := agentsvc.NewServiceContextWithAuthAndPythonExecutor(
		agentRepo,
		logic.NewUserLogicAccountTypeChecker(userLogic),
		cfg.Auth,
		pythonExecutor,
	)
	serviceContext.ConfigureAgentAssembly(userRepo, userRepo, agentRegistryRepo)
	if config.ResolveStorageDriver(cfg.StorageDriver) == config.StorageDriverPostgres {
		authRepo, err := authrepo.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
		if err != nil {
			log.Fatalf("build auth repository: %v", err)
		}
		serviceContext.AuthSessions = authRepo
	} else {
		log.Printf("active session shared validation disabled for storage driver %q; use postgres for single-device enforcement across services", config.ResolveStorageDriver(cfg.StorageDriver))
	}
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg), rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	handler.RegisterAgentGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
