package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// user-rpc 已转为 Postgres-only：accounts/profiles 数据层走 goctl model
	// （service/user/rpc/internal/model），不再支持 memory driver。
	DataSource string `json:",optional"`
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}
