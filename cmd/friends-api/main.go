package main

import (
	"flag"
	"log"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func main() {
	configFile := flag.String("f", "etc/friends-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	serviceContext := svc.NewServiceContext(repository.NewMemoryRepository())
	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(config.ToRestConf(cfg))
	defer server.Stop()
	handler.RegisterFriendsGoZeroHandlers(server, serviceContext)

	log.Printf("%s listening on %s:%d", cfg.Name, cfg.Host, cfg.Port)
	server.Start()
}
