package main

import (
	"flag"
	"fmt"

	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configFile := flag.String("f", "etc/msg-rpc.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts the msg-rpc service: it loads config and serves.
// tracing 由 go-zero 自带 Telemetry（yaml 配置）驱动：zrpc 内置 otel 拦截器 + ServiceConf 启动 trace agent。
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		msgpb.RegisterMsgServer(grpcServer, server.NewMsgServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
