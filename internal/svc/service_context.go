package svc

import (
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	UserLogic    *logic.UserLogic
	FriendsLogic *logic.FriendsLogic
	GroupsLogic  *logic.GroupsLogic
	MessageLogic *logic.MessageLogic
	Repo         repository.Repository
	GroupsRepo   repository.GroupsRepository
	MessageRepo  repository.MessageRepository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	messageRepo := repository.NewMemoryMessageRepository()
	return &ServiceContext{
		UserLogic:    userLogic,
		FriendsLogic: logic.NewFriendsLogic(repo, userLogic),
		MessageLogic: logic.NewMessageLogicWithValidators(messageRepo, logic.NewUserLogicExistenceChecker(userLogic), nil),
		Repo:         repo,
		MessageRepo:  messageRepo,
	}
}

func NewGroupsServiceContext(repo repository.GroupsRepository, userExists logic.UserExistenceChecker) *ServiceContext {
	return &ServiceContext{
		GroupsLogic: logic.NewGroupsLogic(repo, userExists),
		GroupsRepo:  repo,
	}
}

func NewMessageServiceContext(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister) *ServiceContext {
	return &ServiceContext{
		MessageLogic: logic.NewMessageLogicWithValidators(repo, userExists, groups),
		MessageRepo:  repo,
	}
}
