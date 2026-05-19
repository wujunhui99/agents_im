package admin

import (
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type Dependencies struct {
	Accounts    repository.AdminAccountRepository
	Friends     repository.FriendshipRepository
	Messages    repository.AdminMessageRepository
	AgentAudits repository.AdminAgentAuditRepository
}

type ServiceContext struct {
	common.AuthRuntime
	AdminLogic  *logic.AdminLogic
	Accounts    repository.AdminAccountRepository
	Friends     repository.FriendshipRepository
	Messages    repository.AdminMessageRepository
	AgentAudits repository.AdminAgentAuditRepository
}

func NewServiceContext(deps Dependencies) *ServiceContext {
	return NewServiceContextWithAuth(deps, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(deps Dependencies, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		AuthRuntime: common.NewAuthRuntime(auth),
		AdminLogic: logic.NewAdminLogic(logic.AdminLogicConfig{
			Accounts:    deps.Accounts,
			Friends:     deps.Friends,
			Messages:    deps.Messages,
			AgentAudits: deps.AgentAudits,
		}),
		Accounts:    deps.Accounts,
		Friends:     deps.Friends,
		Messages:    deps.Messages,
		AgentAudits: deps.AgentAudits,
	}
}
