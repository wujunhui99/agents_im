package svc

import (
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
	repo := repository.NewMemoryRepository()
	return &ServiceContext{
		Config:    c,
		UserLogic: business.NewUserLogic(repo),
		Repo:      repo,
	}
}
