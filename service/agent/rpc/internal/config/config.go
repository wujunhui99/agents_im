package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config 是 agent-rpc 的配置：既是 gRPC server（AI 托管开关 CRUD，04-agent §3.2），
// 也是 agent.trigger.v1 的 Kafka 消费者 worker（D15 终判 → runtime → 写回，§4.2）。
// tracing 用 go-zero 自带 Telemetry（在 RpcServerConf.ServiceConf 内，由 yaml 配置）。
type Config struct {
	zrpc.RpcServerConf

	// DataSource 是 agent 域自有数据层（goctl model：agent registry / audit / hosting /
	// conv_hosting）。跨域 message 历史读经 msg-rpc gRPC（#617）。Postgres-only。
	DataSource string `json:",optional"`

	// MsgRPC：AI 回复写回经属主 msg-rpc gRPC SendMessage（imadapter，D15 step ④）。
	// AI 消息走与人类消息完全相同的 Kafka 链路，由消费端递归闸门防再触发。
	MsgRPC zrpc.RpcClientConf `json:",optional"`

	// UserRPC：agent-create 工具路径的账号读/建经属主 user-rpc（#606，脱 internal accountRepo）。
	UserRPC zrpc.RpcClientConf `json:",optional"`

	// FriendsRPC：agent-create 工具路径建好友经属主 friends-rpc EnsureFriendship（#606，
	// 取代 internal/repository.EnsureAcceptedFriendship；单向叶子调用，不成环）。
	FriendsRPC zrpc.RpcClientConf `json:",optional"`

	// GroupsRPC：AI 托管 runtime 群成员鉴权经属主 groups-rpc ListMembers（#617，取代
	// internal GroupsLogic 直读；单向叶子调用，不成环）。
	GroupsRPC zrpc.RpcClientConf `json:",optional"`

	// DeepSeek / LLMObservability / PythonExecutor：runtime 与工具配置。
	// DeepSeek/LLMObservability 已搬到本域（#663），#664 改 struct tag 声明式默认值/env：
	// 不标 optional 让 go-zero 在 yaml 缺整块时仍下钻填子字段默认值（子字段各自 optional/default）。
	// PythonExecutor 仍在 pkg/config（第三类待迁）。
	DeepSeek         DeepSeekConfig
	LLMObservability LLMObservabilityConfig
	PythonExecutor   appconfig.PythonExecutorConfig `json:",optional"`

	// Kafka：agent.trigger.v1 消费链路（独立 consumer group，与已退役的 msg-rpc
	// 回流 consumer 隔离）。
	Kafka KafkaConf `json:",optional"`
}

type KafkaConf struct {
	// Brokers 是逗号分隔的 bootstrap 列表；env KAFKA_BROKERS 覆盖。
	Brokers string `json:",optional"`
	// Group 是 agent.trigger.v1 的消费组。
	Group string `json:",default=agent-trigger"`
}
