package config

import (
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	StorageDriver string                      `json:",default=memory,options=memory|postgres|postgresql"`
	DataSource    string                      `json:",optional"`
	Tracing       observability.TracingConfig `json:",optional"`
}
