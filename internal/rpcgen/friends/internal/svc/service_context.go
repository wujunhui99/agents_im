package svc

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/config"
)

type ServiceContext struct {
	Config       config.Config
	FriendsLogic *business.FriendsLogic
	UserLogic    *business.UserLogic
	Repo         repository.Repository
}

func NewServiceContext(c config.Config) *ServiceContext {
	repo := repository.MustRepositoryForStorage(c.StorageDriver, c.DataSource)
	userLogic := business.NewUserLogic(repo)
	return &ServiceContext{
		Config:       c,
		FriendsLogic: business.NewFriendsLogic(repo, userLogic),
		UserLogic:    userLogic,
		Repo:         repo,
	}
}
