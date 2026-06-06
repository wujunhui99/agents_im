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
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	Tracing observability.TracingConfig `json:",optional"`
	Redis   appconfig.RedisConfig       `json:",optional"`
	UserRPC zrpc.RpcClientConf
}
