package svc

import (
	"log"

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
	userRepo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build user repository: %v", err)
	}
	groupsRepo, err := repository.NewGroupsRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	userLogic := business.NewUserLogic(userRepo)
	return &ServiceContext{
		Config:      c,
		GroupsLogic: business.NewGroupsLogic(groupsRepo, business.NewUserLogicExistenceChecker(userLogic)),
		UserLogic:   userLogic,
		GroupsRepo:  groupsRepo,
	}
}
