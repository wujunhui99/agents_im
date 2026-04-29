package svc

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	Auth         config.JWTAuthConfig
	UserLogic    *logic.UserLogic
	FriendsLogic *logic.FriendsLogic
	GroupsLogic  *logic.GroupsLogic
	MessageLogic *logic.MessageLogic
	Repo         repository.Repository
	GroupsRepo   repository.GroupsRepository
	MessageRepo  repository.MessageRepository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	return NewServiceContextWithAuth(repo, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.Repository, auth config.JWTAuthConfig) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	messageRepo := repository.NewMemoryMessageRepository()
	return &ServiceContext{
		Auth:         normalizeAuthConfig(auth),
		UserLogic:    userLogic,
		FriendsLogic: logic.NewFriendsLogic(repo, userLogic),
		MessageLogic: logic.NewMessageLogicWithValidators(messageRepo, logic.NewUserLogicExistenceChecker(userLogic), nil),
		Repo:         repo,
		MessageRepo:  messageRepo,
	}
}

func NewGroupsServiceContext(repo repository.GroupsRepository, userExists logic.UserExistenceChecker) *ServiceContext {
	return NewGroupsServiceContextWithAuth(repo, userExists, config.DefaultJWTAuthConfig())
}

func NewGroupsServiceContextWithAuth(repo repository.GroupsRepository, userExists logic.UserExistenceChecker, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		Auth:        normalizeAuthConfig(auth),
		GroupsLogic: logic.NewGroupsLogic(repo, userExists),
		GroupsRepo:  repo,
	}
}

func NewMessageServiceContext(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister) *ServiceContext {
	return NewMessageServiceContextWithAuth(repo, userExists, groups, config.DefaultJWTAuthConfig())
}

func NewMessageServiceContextWithAuth(repo repository.MessageRepository, userExists logic.UserExistenceChecker, groups logic.GroupMemberLister, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		Auth:         normalizeAuthConfig(auth),
		MessageLogic: logic.NewMessageLogicWithValidators(repo, userExists, groups),
		MessageRepo:  repo,
	}
}

func normalizeAuthConfig(auth config.JWTAuthConfig) config.JWTAuthConfig {
	defaults := config.DefaultJWTAuthConfig()
	if auth.AccessSecret == "" {
		auth.AccessSecret = defaults.AccessSecret
	}
	if auth.AccessExpire <= 0 {
		auth.AccessExpire = defaults.AccessExpire
	}
	return auth
}
