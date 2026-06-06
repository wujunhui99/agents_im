package main

import (
	"flag"
	"fmt"

	"github.com/wujunhui99/agents_im/service/third/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/third/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/third/rpc/internal/svc"
	mailpb "github.com/wujunhui99/agents_im/service/third/rpc/mail"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configFile := flag.String("f", "etc/third-rpc.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts the third-rpc service (third-party integrations: mail/SES): it loads config and serves.
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		mailpb.RegisterMailServiceServer(grpcServer, server.NewMailServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
