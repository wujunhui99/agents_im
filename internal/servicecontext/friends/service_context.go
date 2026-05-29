package friends

import (
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	FriendsLogic *logic.FriendsLogic
	UserLogic    *logic.UserLogic
	Repo         repository.Repository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	return NewServiceContextWithAuth(repo, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.Repository, auth config.JWTAuthConfig) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	return &ServiceContext{
		AuthRuntime:  common.NewAuthRuntime(auth),
		FriendsLogic: logic.NewFriendsLogic(repo, userLogic),
		UserLogic:    userLogic,
		Repo:         repo,
	}
}
