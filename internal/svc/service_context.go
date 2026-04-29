package svc

import (
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	UserLogic    *logic.UserLogic
	FriendsLogic *logic.FriendsLogic
	GroupsLogic  *logic.GroupsLogic
	Repo         repository.Repository
	GroupsRepo   repository.GroupsRepository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	return &ServiceContext{
		UserLogic:    userLogic,
		FriendsLogic: logic.NewFriendsLogic(repo, userLogic),
		Repo:         repo,
	}
}

func NewGroupsServiceContext(repo repository.GroupsRepository, userExists logic.UserExistenceChecker) *ServiceContext {
	return &ServiceContext{
		GroupsLogic: logic.NewGroupsLogic(repo, userExists),
		GroupsRepo:  repo,
	}
}
