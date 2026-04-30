package svc

import (
	"log"

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
	repo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build friends repository: %v", err)
	}
	userLogic := business.NewUserLogic(repo)
	return &ServiceContext{
		Config:       c,
		FriendsLogic: business.NewFriendsLogic(repo, userLogic),
		UserLogic:    userLogic,
		Repo:         repo,
	}
}
