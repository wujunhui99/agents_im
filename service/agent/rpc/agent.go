// agent-rpc — agent 域服务（04-agent §3.2/§4.2，#340）。双角色单进程：
//   - gRPC server：AI 托管开关 CRUD（Get/UpdateConversationAIHosting，数据 owner = agent 域）；
//   - agent.trigger.v1 消费 worker（独立 consumer group）：D15 三步终判（递归闸门 →
//     agent 收信 → conversation_ai_hosting 托管）→ runtime（LLM + tools）→ imadapter
//     经 msg-rpc gRPC 写回 AI 消息（再走完整 Kafka 链，递归闸门拦截 → 终止）。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/server"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/svc"
)

func main() {
	configFile := flag.String("f", "etc/agent.yaml", "the config file")
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx := svc.NewServiceContext(c)

	ensureCtx, cancelEnsure := context.WithTimeout(context.Background(), 30*time.Second)
	if err := messaging.EnsureTopics(ensureCtx, ctx.KafkaBrokers, messaging.TopicAgentTrigger); err != nil {
		cancelEnsure()
		log.Fatalf("ensure kafka topics: %v", err)
	}
	cancelEnsure()

	consumerCtx, cancelConsumer := context.WithCancel(context.Background())
	defer cancelConsumer()
	go func() {
		kafkaConsumer, err := messaging.NewKafkaConsumer(ctx.KafkaBrokers, ctx.KafkaGroup, []string{messaging.TopicAgentTrigger})
		if err != nil {
			logx.Errorf("agent trigger consumer not started: %v", err)
			return
		}
		defer kafkaConsumer.Close()
		if err := kafkaConsumer.Run(consumerCtx, ctx.Consumer.HandleBatch); err != nil && consumerCtx.Err() == nil {
			logx.Errorf("agent trigger consumer stopped: %v", err)
		}
	}()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		agent.RegisterAgentServer(grpcServer, server.NewAgentServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	// 统一记录 server-fault 错误（带 trace_id），避免 handler 返回 Internal 时服务端沉默（#630）。
	s.AddUnaryInterceptors(observability.ErrorLogUnaryServerInterceptor())
	defer s.Stop()

	fmt.Printf("agent-rpc serving at %s, consuming %s (group=%s)...\n", c.ListenOn, messaging.TopicAgentTrigger, ctx.KafkaGroup)
	s.Start()
}
