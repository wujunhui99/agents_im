package config

import (
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// admin-rpc 是 admin 域唯一碰 DB 的服务：task_reports 走 goctl model
	// （service/admin/rpc/internal/model）、feedback 走 admin goctl model（#678）；
	// 其余跨域只读全部经属主 rpc（user/agent/msg/friends），已脱顶层 internal/repository（#618）。
	DataSource string `json:",optional"`
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。

	// UserRPC：跨域账号只读(用户详情/搜索/计数)经属主 user-rpc
	// （gate #550，脱 internal/repository accountRepo 的 avatar string scan）。
	UserRPC zrpc.RpcClientConf `json:",optional"`
	// AgentRPC：agent 审计 traces/dashboard 只读经属主 agent-rpc gRPC
	// （#616，脱 internal/repository agent_audit 的直读）。
	AgentRPC zrpc.RpcClientConf `json:",optional"`
	// MsgRPC：会话消息只读(会话消息列表/用户会话)与 AI 重放经属主 msg-rpc gRPC
	// （#618，脱 internal/repository AdminMessageRepository + MessageCreatedHook）。
	MsgRPC zrpc.RpcClientConf `json:",optional"`
	// FriendsRPC：好友关系只读经属主 friends-rpc gRPC（#618，脱 internal/repository FriendshipRepository）。
	FriendsRPC zrpc.RpcClientConf `json:",optional"`
}
