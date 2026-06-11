package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/logic"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
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

	// Kafka 写路径（03 §9 B2，MSG_DIRECT_KAFKA）：绑定 AI 写回 + 启动 agent.trigger
	// 消费（触发点已前移 msgtransfer，经 Kafka 回流到本进程 AgentHook）。
	if ctx.KafkaEnabled {
		ctx.BindAgentResponseSender(logic.NewAgentResponseSender(ctx))

		ensureCtx, cancelEnsure := context.WithTimeout(context.Background(), 30*time.Second)
		if err := messaging.EnsureTopics(ensureCtx, ctx.KafkaBrokers,
			messaging.TopicToTransfer, messaging.TopicAgentTrigger); err != nil {
			cancelEnsure()
			log.Fatalf("ensure kafka topics (MSG_DIRECT_KAFKA on): %v", err)
		}
		cancelEnsure()

		consumerCtx, cancelConsumer := context.WithCancel(context.Background())
		defer cancelConsumer()
		go func() {
			if err := logic.RunAgentTriggerConsumer(consumerCtx, ctx); err != nil && consumerCtx.Err() == nil {
				logx.Errorf("agent trigger consumer stopped: %v", err)
			}
		}()
		fmt.Printf("MSG_DIRECT_KAFKA on: producing %s, consuming %s\n", messaging.TopicToTransfer, messaging.TopicAgentTrigger)
	}

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
