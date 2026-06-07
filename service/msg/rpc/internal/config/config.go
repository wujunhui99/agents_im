package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// msg-rpc 数据层走 goctl model（Postgres-only），不再支持 memory driver。
	// tracing 用 go-zero 自带 Telemetry（在 RpcServerConf.ServiceConf 内，由 yaml 配置）。
	DataSource string `json:",optional"`
}
