package svc

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/config"
)

type ServiceContext struct {
	Config      config.Config
	GroupsLogic *business.GroupsLogic
	UserLogic   *business.UserLogic
	GroupsRepo  repository.GroupsRepository
}

func NewServiceContext(c config.Config) *ServiceContext {
	userLogic := business.NewUserLogic(repository.NewMemoryRepository())
	groupsRepo := repository.NewMemoryGroupsRepository()
	return &ServiceContext{
		Config:      c,
		GroupsLogic: business.NewGroupsLogic(groupsRepo, business.NewUserLogicExistenceChecker(userLogic)),
		UserLogic:   userLogic,
		GroupsRepo:  groupsRepo,
	}
}
