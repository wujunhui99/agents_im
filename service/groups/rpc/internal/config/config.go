package config

import (
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// groups-rpc 已转为 Postgres-only，数据层走 goctl model，不再支持 memory driver。
	DataSource string                      `json:",optional"`
	Tracing    observability.TracingConfig `json:",optional"`
}
