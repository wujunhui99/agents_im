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
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/groups-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	userRepo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	groupsRepo, err := repository.NewGroupsRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	userLogic := logic.NewUserLogic(userRepo)
	serviceContext := groupssvc.NewServiceContextWithAuth(
		groupsRepo,
		logic.NewUserLogicExistenceChecker(userLogic),
		cfg.Auth,
	)
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
	handler.RegisterGroupsGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
