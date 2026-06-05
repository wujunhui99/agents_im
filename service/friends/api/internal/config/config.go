// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	Tracing    observability.TracingConfig `json:",optional"`
	FriendsRPC zrpc.RpcClientConf
	// UserRPC：BFF 聚合用，补全好友资料（friends rpc 不再跨域读用户表）。
	UserRPC zrpc.RpcClientConf
}
