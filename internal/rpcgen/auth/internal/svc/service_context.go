package svc

import (
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
		AuthLogic: business.NewAuthLogic(authRepo, useradapter.NewLogicClient(userLogic), business.NewPasswordHasher(), token.NewHMACTokenManager(c.Auth.AccessSecret, time.Duration(c.Auth.AccessExpire)*time.Second)),
		AuthRepo:  authRepo,
		UserLogic: userLogic,
	}
}
