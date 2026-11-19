// resource-rpc 统一管理外部资源：对象存储生命周期（media，原 media-rpc）与外部
// 供应商调用（mail/SES，原 third-rpc）。两个能力域在同一个进程内注册为两个 gRPC
// service，共享端口、Telemetry 与生命周期。
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/resource/rpc/internal/config"
	mailserver "github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/server"
	medialogic "github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/logic"
	mediaserver "github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/server"
	"github.com/wujunhui99/agents_im/service/resource/rpc/internal/svc"
	mailpb "github.com/wujunhui99/agents_im_protocol/mail/v1"
	mediapb "github.com/wujunhui99/agents_im_protocol/media/v1"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	configFile := flag.String("f", "etc/resource-rpc.yaml", "the config file")
	flag.Parse()
	run(*configFile)
}

// run starts resource-rpc: it loads config and serves both media and mail services.
func run(configFile string) {
	var c config.Config
	conf.MustLoad(configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	// 后台清理 tmp/ 残留（中断/失败上传的孤儿对象，EPIC #527 §3）；随进程生命周期运行。
	if ctx.Media.Store != nil {
		go medialogic.NewTempUploadCleaner(ctx.Media.Store).Run(context.Background())
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		mediapb.RegisterMediaServer(grpcServer, mediaserver.NewMediaServer(ctx.Media))
		mailpb.RegisterMailServer(grpcServer, mailserver.NewMailServer(ctx.Mail))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// 统一记录 server-fault 错误（带 trace_id），避免 handler 返回 Internal 时服务端沉默（#630）。
	s.AddUnaryInterceptors(observability.ErrorLogUnaryServerInterceptor())
	defer s.Stop()

	fmt.Printf("Starting resource-rpc server at %s...\n", c.ListenOn)
	s.Start()
}
