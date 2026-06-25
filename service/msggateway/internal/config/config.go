package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	Name        string
	Host        string
	Port        int
	Auth        appconfig.JWTAuthConfig
	Tracing     observability.TracingConfig
	MsgRPC      zrpc.RpcClientConf
	Presence    appconfig.PresenceConfig
	Redis       appconfig.RedisConfig
	GatewayWS   appconfig.GatewayWSConfig
	GatewayGRPC appconfig.GatewayGRPCConfig
}
