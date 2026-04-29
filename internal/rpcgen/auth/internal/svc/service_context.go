package svc

import (
	"os"
	"strconv"
	"time"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/config"
)

type ServiceContext struct {
	Config    config.Config
	AuthLogic *business.AuthLogic
	AuthRepo  authrepo.CredentialRepository
	UserLogic *userlogic.UserLogic
}

func NewServiceContext(c config.Config) *ServiceContext {
	userLogic := userlogic.NewUserLogic(userrepo.NewMemoryRepository())
	authRepo := authrepo.NewMemoryRepository()
	return &ServiceContext{
		Config:    c,
		AuthLogic: business.NewAuthLogic(authRepo, useradapter.NewLogicClient(userLogic), business.NewPasswordHasher(), token.NewHMACTokenManager(tokenSecret(), tokenTTL())),
		AuthRepo:  authRepo,
		UserLogic: userLogic,
	}
}

func tokenSecret() string {
	if value := os.Getenv("AUTH_TOKEN_SECRET"); value != "" {
		return value
	}
	return "dev-auth-secret-change-me"
}

func tokenTTL() time.Duration {
	value := os.Getenv("AUTH_TOKEN_TTL")
	if value == "" {
		return 24 * time.Hour
	}

	ttl, err := time.ParseDuration(value)
	if err == nil {
		return ttl
	}

	seconds, err := strconv.Atoi(value)
	if err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	return 24 * time.Hour
}
