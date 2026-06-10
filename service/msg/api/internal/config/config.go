// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	// tracing 用 go-zero 自带 Telemetry（在 RestConf.ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
	Redis    appconfig.RedisConfig `json:",optional"`
	MsgRPC   zrpc.RpcClientConf
	AdminRPC zrpc.RpcClientConf
}
