package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	friendsrpc "github.com/wujunhui99/agents_im/internal/rpc"
)

func main() {
	configFile := flag.String("f", "etc/friends-rpc.yaml", "config file")
	flag.Parse()

	cfg, err := config.LoadRPCConfig(*configFile)
	if err != nil {
		log.Fatalf("load rpc config: %v", err)
	}

	repo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(repo)
	friendsLogic := logic.NewFriendsLogic(repo, userLogic)
	_ = friendsrpc.NewFriendsServer(friendsLogic)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"code":"OK","message":"ok","data":{"status":"friends rpc contract initialized"}}` + "\n"))
	})

	log.Printf("%s contract initialized on %s; gRPC transport should be generated with goctl/protoc when available", cfg.Name, cfg.ListenOn)
	if err := http.ListenAndServe(cfg.ListenOn, mux); err != nil {
		log.Fatal(err)
	}
}
