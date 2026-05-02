package main

import (
	"context"
	"flag"
	"log"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/objectstorage"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
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

	repo, err := repository.NewRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(cfg.StorageDriver, cfg.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	objectStore, err := objectstorage.NewStore(cfg.ObjectStorage)
	if err != nil {
		log.Fatalf("build object store: %v", err)
	}
	if err := objectStore.EnsureBucket(context.Background()); err != nil {
		log.Fatalf("ensure object storage bucket: %v", err)
	}
	serviceContext := svc.NewUserServiceContextWithMedia(repo, mediaRepo, objectStore, cfg.ObjectStorage.Bucket, cfg.Auth)
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg), rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	defer server.Stop()
	server.Use(observability.TraceMiddlewareFunc)
	handler.RegisterUserGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
