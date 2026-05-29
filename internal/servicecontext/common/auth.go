package common

import (
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/pkg/config"
)

type AuthRuntime struct {
	Auth         config.JWTAuthConfig
	AuthSessions authrepo.ActiveSessionRepository
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

func (r AuthRuntime) ActiveSessionRepository() authrepo.ActiveSessionRepository {
	return r.AuthSessions
}
