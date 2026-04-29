package main

import (
	"flag"
	"fmt"

	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/config"
	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/server"
	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/svc"
	"github.com/wujunhui99/agents_im/proto/messagepb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/message.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		messagepb.RegisterMessageServiceServer(grpcServer, server.NewMessageServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
