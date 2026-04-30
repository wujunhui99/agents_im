package svc

import (
	"log"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/rpcgen/user/internal/config"
)

type ServiceContext struct {
	Config    config.Config
	UserLogic *business.UserLogic
	Repo      repository.Repository
}

func NewServiceContext(c config.Config) *ServiceContext {
	repo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	return &ServiceContext{
		Config:    c,
		UserLogic: business.NewUserLogic(repo),
		Repo:      repo,
	}
}
