package config

import (
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
	Tracing  observability.TracingConfig `json:",optional"`
	MediaRPC zrpc.RpcClientConf
}
