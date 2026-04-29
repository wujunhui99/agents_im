package svc

import (
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
)

type ServiceContext struct {
	AuthLogic *authlogic.AuthLogic
	AuthRepo  authrepo.CredentialRepository
	Users     useradapter.UserClient
}

func NewServiceContext(repo authrepo.CredentialRepository, users useradapter.UserClient, tokenManager token.Manager) *ServiceContext {
	return &ServiceContext{
		AuthLogic: authlogic.NewAuthLogic(repo, users, authlogic.NewPasswordHasher(), tokenManager),
		AuthRepo:  repo,
		Users:     users,
	}
}
