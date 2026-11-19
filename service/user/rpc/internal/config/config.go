package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// user-rpc 已转为 Postgres-only：accounts/profiles 数据层走 goctl model
	// （service/user/rpc/internal/model），不再支持 memory driver。
	DataSource string `json:",optional"`
	// ResourceRPC 用于头像 media 校验（#533，取代 internal/mediavalidate 直读 media_objects）。
	ResourceRPC zrpc.RpcClientConf
	// Kafka 用于发布 user.created.v1：新账号的跨域开通（默认助手绑定 + 好友）
	// 由 agent-rpc 消费事件完成，user-rpc 不再同步调用 agent-rpc/friends-rpc。
	Kafka KafkaConfig `json:",optional"`
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}

// KafkaConfig 是 user-rpc 的事件出口配置（只在创建账号后 publish user.created.v1）。
type KafkaConfig struct {
	Brokers string `json:",optional"`
}
