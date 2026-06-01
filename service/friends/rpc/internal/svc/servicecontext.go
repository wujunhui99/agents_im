package svc

import (
	"log"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/friends/core"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/config"
)

type ServiceContext struct {
	Config       config.Config
	FriendsLogic *core.FriendsLogic
	Repo         repository.Repository
}

func NewServiceContext(c config.Config) *ServiceContext {
	repo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build friends repository: %v", err)
	}
	return &ServiceContext{
		Config:       c,
		FriendsLogic: core.NewFriendsLogic(repo, core.NewAccountRepoUserLookup(repo)),
		Repo:         repo,
	}
}
