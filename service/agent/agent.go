// service/agent — agent-domain trigger consumer (04-agent §4.2, D15 step ③).
//
// Consumes agent.trigger.v1 with its own consumer group ("agent-trigger") and
// performs the full final judgment there: recursion gate → agent-inbox by D16
// account-id type bits → conversation hosting. Interface-first scaffold
// (issue #503): runtime / write-back / hosting are explicit mock drivers, so
// this service produces NO side effects yet — the transitional msg-rpc 回流
// consumer keeps owning real AI replies until D15 step ④ swaps ownership.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/zeromicro/go-zero/core/conf"

	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	agentconfig "github.com/wujunhui99/agents_im/service/agent/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/internal/consumer"
	"github.com/wujunhui99/agents_im/service/agent/internal/imadapter"
	"github.com/wujunhui99/agents_im/service/agent/internal/runtime"
	"github.com/wujunhui99/agents_im/service/agent/internal/trigger"
)

func main() {
	configFile := flag.String("f", "etc/agent.yaml", "config file")
	flag.Parse()

	var cfg agentconfig.Config
	conf.MustLoad(*configFile, &cfg)

	brokers := config.KafkaBrokerList(firstNonEmpty(
		strings.TrimSpace(os.Getenv("KAFKA_BROKERS")),
		strings.TrimSpace(os.ExpandEnv(cfg.Kafka.Brokers)),
	))
	if len(brokers) == 0 {
		log.Fatalf("%s: no Kafka brokers configured (set Kafka.Brokers or KAFKA_BROKERS)", cfg.Name)
	}

	pipeline, err := buildPipeline(cfg)
	if err != nil {
		log.Fatalf("%s: %v", cfg.Name, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	ensureCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	err = messaging.EnsureTopics(ensureCtx, brokers, messaging.TopicAgentTrigger)
	cancel()
	if err != nil {
		log.Fatalf("%s: ensure topics: %v", cfg.Name, err)
	}

	kafkaConsumer, err := messaging.NewKafkaConsumer(brokers, cfg.Kafka.Group, []string{messaging.TopicAgentTrigger})
	if err != nil {
		log.Fatalf("%s: new kafka consumer: %v", cfg.Name, err)
	}
	defer kafkaConsumer.Close()

	log.Printf("%s consuming topic=%s group=%s brokers=%s runtime=%s sender=%s hosting=%s",
		cfg.Name, messaging.TopicAgentTrigger, cfg.Kafka.Group, strings.Join(brokers, ","),
		cfg.Runtime.Driver, cfg.Sender.Driver, cfg.Hosting.Driver)
	if err := kafkaConsumer.Run(ctx, pipeline.HandleBatch); err != nil && ctx.Err() == nil {
		log.Fatalf("%s: consumer stopped: %v", cfg.Name, err)
	}
	log.Printf("%s stopped", cfg.Name)
}

// buildPipeline resolves the configured drivers. Only the explicit mock
// drivers exist in the scaffold; anything else fails fast instead of silently
// falling back (AGENTS.md 失败优先).
func buildPipeline(cfg agentconfig.Config) (*consumer.Consumer, error) {
	var hosting trigger.HostingStore
	switch cfg.Hosting.Driver {
	case agentconfig.DriverMock:
		hosting = trigger.NewMockHostingStore(nil)
	default:
		return nil, fmt.Errorf("unsupported Hosting.Driver %q (scaffold supports only %q)", cfg.Hosting.Driver, agentconfig.DriverMock)
	}
	judge, err := trigger.NewJudge(hosting)
	if err != nil {
		return nil, err
	}

	var rt runtime.Runtime
	switch cfg.Runtime.Driver {
	case agentconfig.DriverMock:
		rt = runtime.NewMock()
	default:
		return nil, fmt.Errorf("unsupported Runtime.Driver %q (scaffold supports only %q)", cfg.Runtime.Driver, agentconfig.DriverMock)
	}

	var sender imadapter.MessageSender
	switch cfg.Sender.Driver {
	case agentconfig.DriverMock:
		sender = imadapter.NewMock()
	default:
		return nil, fmt.Errorf("unsupported Sender.Driver %q (scaffold supports only %q)", cfg.Sender.Driver, agentconfig.DriverMock)
	}

	return consumer.New(judge, rt, sender)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
