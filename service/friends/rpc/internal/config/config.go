package config

import (
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// friends-rpc 已转为 Postgres-only，数据层走 goctl model（service/friends/rpc/internal/model），
	// 不再支持 memory driver，也不再依赖顶层 internal/repository。
	DataSource string                      `json:",optional"`
	Tracing    observability.TracingConfig `json:",optional"`
}
