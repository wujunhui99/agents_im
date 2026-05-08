package groups

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	GroupsLogic *logic.GroupsLogic
	GroupsRepo  repository.GroupsRepository
}

func NewServiceContext(repo repository.GroupsRepository, userExists logic.UserExistenceChecker) *ServiceContext {
	return NewServiceContextWithAuth(repo, userExists, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.GroupsRepository, userExists logic.UserExistenceChecker, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		AuthRuntime: common.NewAuthRuntime(auth),
		GroupsLogic: logic.NewGroupsLogic(repo, userExists),
		GroupsRepo:  repo,
	}
}
