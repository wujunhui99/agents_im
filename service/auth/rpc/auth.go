package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configFile := flag.String("f", "etc/auth-rpc.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts the auth-rpc service: it loads config and serves.
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	if c.TokenAuth.AccessSecret == "" {
		c.TokenAuth.AccessSecret = appconfig.DefaultJWTAuthConfig().AccessSecret
	}
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
