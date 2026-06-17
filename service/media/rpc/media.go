package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/wujunhui99/agents_im/service/media/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/logic"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/media/rpc/internal/svc"
	mediapb "github.com/wujunhui99/agents_im/service/media/rpc/media"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configFile := flag.String("f", "etc/media-rpc.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts the media-rpc service: it loads config and serves.
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	// 后台清理 tmp/ 残留（中断/失败上传的孤儿对象，EPIC #527 §3）；随进程生命周期运行。
	if ctx.Store != nil {
		go logic.NewTempUploadCleaner(ctx.Store).Run(context.Background())
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		mediapb.RegisterMediaServer(grpcServer, server.NewMediaServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
