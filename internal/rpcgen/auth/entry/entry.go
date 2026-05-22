package entry

import (
	"context"
	"fmt"
	"log"

	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/config"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/server"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/svc"
	"github.com/wujunhui99/agents_im/proto/authpb"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Start bridges cmd/auth-rpc to the goctl-generated RPC internals.
// cmd/auth-rpc cannot import internal/rpcgen/auth/internal/* directly because
// of Go internal package visibility.
func Start(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c)
	c.Telemetry = appconfig.GoZeroTelemetryConfig(c.Tracing, c.Name)
	shutdownTracing, err := observability.InitServiceTracing(context.Background(), c.Tracing, c.Name)
	if err != nil {
		log.Fatalf("init tracing: %v", err)
	}
	defer func() {
		if err := observability.ShutdownTracing(shutdownTracing); err != nil {
			log.Printf("shutdown tracing: %v", err)
		}
	}()
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		authpb.RegisterAuthServiceServer(grpcServer, server.NewAuthServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()
	s.AddUnaryInterceptors(observability.GRPCUnaryServerInterceptor())

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
