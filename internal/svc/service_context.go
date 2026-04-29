package svc

import (
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	UserLogic   *logic.UserLogic
	GroupsLogic *logic.GroupsLogic
	Repo        repository.UserRepository
	GroupsRepo  repository.GroupsRepository
}

func NewServiceContext(repo repository.UserRepository) *ServiceContext {
	return &ServiceContext{
		UserLogic: logic.NewUserLogic(repo),
		Repo:      repo,
	}
}

func NewGroupsServiceContext(repo repository.GroupsRepository, userExists logic.UserExistenceChecker) *ServiceContext {
	return &ServiceContext{
		GroupsLogic: logic.NewGroupsLogic(repo, userExists),
		GroupsRepo:  repo,
	}
}
