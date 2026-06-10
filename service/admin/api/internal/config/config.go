// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
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
	Redis appconfig.RedisConfig `json:",optional"`
	// AdminRPC：admin-api 改纯 BFF 后，所有 DB 访问都走 admin-rpc（admin 域唯一碰 DB 的服务）。
	AdminRPC zrpc.RpcClientConf
	// UserRPC / AuthRPC：BFF 编排「创建测试账户」——user-rpc 建号（type=test），
	// auth-rpc 设登录凭据；跨域写不进 admin-rpc。
	UserRPC zrpc.RpcClientConf
	AuthRPC zrpc.RpcClientConf
}
