package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// friends-rpc 已转为 Postgres-only，数据层走 goctl model（service/friends/rpc/internal/model），
	// 不再支持 memory driver，也不再依赖顶层 internal/repository。
	DataSource string `json:",optional"`
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}
