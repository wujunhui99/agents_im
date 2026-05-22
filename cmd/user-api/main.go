package main

import (
	"context"
	"flag"
	"log"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/user-api.yaml", "config file")
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

	repo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	messageRepo, err := repository.NewMessageRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	agentRepo, err := repository.NewAgentRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent repository: %v", err)
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build agent registry repository: %v", err)
	}
	objectStore, err := objectstorage.NewStore(cfg.ObjectStorage)
	if err != nil {
		log.Fatalf("build object store: %v", err)
	}
	if err := objectStore.EnsureBucket(context.Background()); err != nil {
		log.Fatalf("ensure object storage bucket: %v", err)
	}
	serviceContext := usersvc.NewServiceContextWithMedia(repo, mediaRepo, objectStore, cfg.ObjectStorage.Bucket, cfg.Auth)
	serviceContext.ConfigureDefaultAssistant(agentRepo, agentRegistryRepo)
	if _, err := serviceContext.DefaultAssistant.Backfill(context.Background()); err != nil {
		log.Fatalf("backfill default assistant: %v", err)
	}
	serviceContext.ConfigureMediaAttachmentAccess(messageRepo)
	if config.ResolveStorageDriver(cfg.StorageDriver) == config.StorageDriverPostgres {
		authRepo, err := authrepo.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
		if err != nil {
			log.Fatalf("build auth repository: %v", err)
		}
		serviceContext.AuthSessions = authRepo
	} else {
		log.Printf("active session shared validation disabled for storage driver %q; use postgres for single-device enforcement across services", config.ResolveStorageDriver(cfg.StorageDriver))
	}
	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(config.ToRestConf(cfg), rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	handler.RegisterUserGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
