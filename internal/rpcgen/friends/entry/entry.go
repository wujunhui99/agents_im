package entry

import (
	"fmt"

	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/config"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/server"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/proto/friendspb"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func Start(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c)
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		friendspb.RegisterFriendsServiceServer(grpcServer, server.NewFriendsServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
