package svc

import (
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type ServiceContext struct {
	UserLogic    *logic.UserLogic
	FriendsLogic *logic.FriendsLogic
	Repo         repository.Repository
}

func NewServiceContext(repo repository.Repository) *ServiceContext {
	userLogic := logic.NewUserLogic(repo)
	return &ServiceContext{
		UserLogic:    userLogic,
		FriendsLogic: logic.NewFriendsLogic(repo, userLogic),
		Repo:         repo,
	}
}
