package config

import (
	mailprovider "github.com/wujunhui99/agents_im/service/third/rpc/internal/provider"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	TencentSES mailprovider.TencentSESConfig
	Tracing    observability.TracingConfig `json:",optional"`
}
