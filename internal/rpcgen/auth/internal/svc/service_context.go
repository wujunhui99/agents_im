package svc

import (
	"log"
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
	userRepo, err := userrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	authRepo, err := authrepo.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build auth repository: %v", err)
	}
	userLogic := userlogic.NewUserLogic(userRepo)
	return &ServiceContext{
		Config:    c,
		AuthLogic: business.NewAuthLogic(authRepo, useradapter.NewLogicClient(userLogic), business.NewPasswordHasher(), token.NewHMACTokenManager(c.JWTAuth.AccessSecret, time.Duration(c.JWTAuth.AccessExpire)*time.Second)),
		AuthRepo:  authRepo,
		UserLogic: userLogic,
	}
}
