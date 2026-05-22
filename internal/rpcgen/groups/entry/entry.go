package entry

import (
	"context"
	"fmt"
	"log"

	appconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/config"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/server"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/svc"
	"github.com/wujunhui99/agents_im/proto/groupspb"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Start bridges cmd/groups-rpc to the goctl-generated RPC internals.
// cmd/groups-rpc cannot import internal/rpcgen/groups/internal/* directly because
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
		groupspb.RegisterGroupsServiceServer(grpcServer, server.NewGroupsServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()
	s.AddUnaryInterceptors(observability.GRPCUnaryServerInterceptor())

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
