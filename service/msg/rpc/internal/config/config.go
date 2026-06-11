package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// msg-rpc 数据层走 goctl model（Postgres-only），不再支持 memory driver。
	// tracing 用 go-zero 自带 Telemetry（在 RpcServerConf.ServiceConf 内，由 yaml 配置）。
	DataSource string `json:",optional"`

	// AI 托管运行时（keystone 例外：随 message-api 退役迁入，SendMessage 后触发 Agent 回复；
	// 待 03-message-pipeline §9 B1 把触发点迁到 msgtransfer 后删除）。
	DeepSeek         appconfig.DeepSeekConfig         `json:",optional"`
	LLMObservability appconfig.LLMObservabilityConfig `json:",optional"`
	PythonExecutor   appconfig.PythonExecutorConfig   `json:",optional"`

	// Kafka 写路径开关（03 §9 B2）：见 KafkaConfig。
	Kafka KafkaConfig `json:",optional"`
}

// KafkaConfig 是 Kafka 写路径开关（03-message-pipeline §9 B2，feature flag
// MSG_DIRECT_KAFKA，可被同名环境变量覆盖，见 svc 装配）。
// on：SendMessage 只 publish message.submitted 到 msg.toTransfer.v1（不写 PG、
// ACK 不带 seq），AI 触发经 agent.trigger.v1 consumer 回流；off：行为与旧实现
// 完全一致（同步写 PG+outbox、ACK 带 seq）。切换即回滚开关（秒级）。
type KafkaConfig struct {
	Enabled bool   `json:",optional"`
	Brokers string `json:",optional"`
}
