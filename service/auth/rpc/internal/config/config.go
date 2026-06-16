package config

import (
	commonconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	TokenAuth     commonconfig.JWTAuthConfig
	StorageDriver string `json:",default=memory,options=memory|postgres|postgresql"`
	DataSource    string `json:",optional"`
	// SessionRedis (not "Redis") avoids colliding with zrpc.RpcServerConf.Redis
	// (redis.RedisKeyConf), which go-zero would otherwise populate from this block
	// and reject for a missing required Host. Holds the active-session store conn.
	SessionRedis commonconfig.RedisConfig `json:",optional"`
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
	MailRPC zrpc.RpcClientConf `json:",optional"`
	// UserRPC：auth 注册/登录读用户资料经属主 user-rpc（#551，脱 internal/logic.UserLogic）。
	UserRPC zrpc.RpcClientConf `json:",optional"`
}
