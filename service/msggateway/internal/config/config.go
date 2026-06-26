package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Name string
	Host string
	Port int
	Auth appconfig.JWTAuthConfig
	// Tracing 由 ConfigMap 注入的 AGENTS_IM_* 环境变量驱动，yaml 不带 Tracing 块；
	// 标 optional 让 conf.MustLoad 缺失时取零值，再由 ResolveTracingConfig 走 env/默认值。
	Tracing  observability.TracingConfig `json:",optional"`
	MsgRPC   zrpc.RpcClientConf
	Presence appconfig.PresenceConfig
	// Redis 同理走 env/默认值：本地 etc/msggateway.yaml 不带 Redis 块，缺失时
	// NewRedisSessionStore 回落到 DefaultRedisConfig().Addr（localhost:6379），保持 #655 前行为。
	Redis       appconfig.RedisConfig `json:",optional"`
	GatewayWS   GatewayWSConfig
	GatewayGRPC GatewayGRPCConfig
}
