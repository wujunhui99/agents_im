package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
	MediaRPC zrpc.RpcClientConf
	// 下载授权编排（BFF 聚合，EPIC #527 §4，#532）：media-api 聚合 msg/friends/groups 做链路校验 +
	// 私聊单向好友 / 群成员判定，再调 media-rpc 纯签发；rpc 之间不互调（见 AGENTS.md 微服务分层约定）。
	MsgRPC     zrpc.RpcClientConf
	FriendsRPC zrpc.RpcClientConf
	GroupsRPC  zrpc.RpcClientConf
}
