package svc

import (
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	UserLogic *logic.UserLogic
	Repo      repository.UserRepository
}

func NewServiceContext(repo repository.UserRepository) *ServiceContext {
	return &ServiceContext{
		UserLogic: logic.NewUserLogic(repo),
		Repo:      repo,
	}
}
