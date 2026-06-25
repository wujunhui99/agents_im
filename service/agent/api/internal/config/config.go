// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Auth appconfig.JWTAuthConfig
	// Tracing 由 ConfigMap 注入的 AGENTS_IM_* 环境变量驱动，yaml 不带 Tracing 块；
	// 标 optional 让 conf.MustLoad 缺失时取零值，再由 ResolveTracingConfig 走 env/默认值。
	Tracing  observability.TracingConfig `json:",optional"`
	AgentRPC zrpc.RpcClientConf
	Redis    appconfig.RedisConfig `json:",optional"`
}
