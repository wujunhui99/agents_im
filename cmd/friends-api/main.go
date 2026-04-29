package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func main() {
	configFile := flag.String("f", "etc/friends-api.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadAPIConfig(*configFile)
	if err != nil {
		log.Fatalf("load api config: %v", err)
	}

	serviceContext := svc.NewServiceContext(repository.NewMemoryRepository())
	mux := http.NewServeMux()
	handler.RegisterFriendsHandlers(mux, serviceContext)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	log.Printf("%s listening on %s", cfg.Name, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
