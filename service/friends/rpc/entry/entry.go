package entry

import (
	"context"
	"fmt"
	"log"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	friendspb "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Start launches the friends-rpc service, wiring the goctl-generated RPC internals. It lives in the entry package so the service
// binary and tests can share one startup path.
func Start(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
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
		friendspb.RegisterFriendsServer(grpcServer, server.NewFriendsServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()
	s.AddUnaryInterceptors(observability.GRPCUnaryServerInterceptor())

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
