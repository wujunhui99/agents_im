package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
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

	// Kafka 唯一写链路（03 §9 B2/B3b）：SendMessage 只 publish msg.toTransfer.v1。
	// AI 触发/运行/写回已整体迁出至 agent-rpc（#340，D15 step ④）：agent-rpc 以独立
	// consumer group 消费 agent.trigger.v1，AI 回复经 imadapter→msg-rpc gRPC SendMessage
	// 写回——本进程不再消费 agent.trigger.v1、不再跑 agent runtime。
	ensureCtx, cancelEnsure := context.WithTimeout(context.Background(), 30*time.Second)
	if err := messaging.EnsureTopics(ensureCtx, ctx.KafkaBrokers, messaging.TopicToTransfer); err != nil {
		cancelEnsure()
		log.Fatalf("ensure kafka topics: %v", err)
	}
	cancelEnsure()

	fmt.Printf("kafka write path: producing %s\n", messaging.TopicToTransfer)

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
