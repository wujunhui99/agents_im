package agent

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	AgentLogic *logic.AgentLogic
	AgentRepo  repository.AgentRepository
}

func NewServiceContext(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker) *ServiceContext {
	return NewServiceContextWithAuth(repo, accountTypeChecker, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		AuthRuntime: common.NewAuthRuntime(auth),
		AgentLogic:  logic.NewAgentLogic(repo, accountTypeChecker),
		AgentRepo:   repo,
	}
}
