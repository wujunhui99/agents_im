// Package authruntime 持有跨服务共享的鉴权运行时（JWT 配置 + 活跃会话存储），供 api/rpc
// 的 ServiceContext 内嵌。原寄居 internal/servicecontext/common（monolith 共享入口），随顶层
// internal/ 退役迁入 pkg/（#618）。
package authruntime

import (
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/middleware"
)

type AuthRuntime struct {
	Auth     config.JWTAuthConfig
	Sessions middleware.SessionStore
}

func NewAuthRuntime(auth config.JWTAuthConfig) AuthRuntime {
	return AuthRuntime{Auth: NormalizeAuthConfig(auth)}
}

func NormalizeAuthConfig(auth config.JWTAuthConfig) config.JWTAuthConfig {
	defaults := config.DefaultJWTAuthConfig()
	if auth.AccessSecret == "" {
		auth.AccessSecret = defaults.AccessSecret
	}
	if auth.AccessExpire <= 0 {
		auth.AccessExpire = defaults.AccessExpire
	}
	return auth
}

func (r AuthRuntime) AuthConfig() config.JWTAuthConfig {
	return r.Auth
}

// SessionStore returns the Redis-backed active-session store used to enforce
// single active session per (user, device). Nil disables shared validation.
func (r AuthRuntime) SessionStore() middleware.SessionStore {
	return r.Sessions
}
