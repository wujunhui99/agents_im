package entry

import (
	"fmt"

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
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		groupspb.RegisterGroupsServiceServer(grpcServer, server.NewGroupsServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
