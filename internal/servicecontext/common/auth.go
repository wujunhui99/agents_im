package common

import (
	"github.com/wujunhui99/agents_im/pkg/middleware"
	"github.com/wujunhui99/agents_im/pkg/config"
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
