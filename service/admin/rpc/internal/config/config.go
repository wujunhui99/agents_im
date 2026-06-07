package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// admin-rpc 是 admin 域唯一碰 DB 的服务，转为 Postgres-only。
	// task_reports 走 goctl model（service/admin/rpc/internal/model）；
	// accounts/friendships/messages/agent_audits/feedback 为跨域只读，
	// 暂经顶层 internal/repository（待相关域 rpc 落地后迁移）。
	DataSource string `json:",optional"`
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}
