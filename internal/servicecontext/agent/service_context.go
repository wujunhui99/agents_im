package agent

import (
	"github.com/wujunhui99/agents_im/internal/agent/pythonexec"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
)

type ServiceContext struct {
	common.AuthRuntime
	AgentLogic           *logic.AgentLogic
	AgentDefinitionLogic *logic.AgentAssemblyLogic
	AgentRepo            repository.AgentRepository
	AgentRegistryRepo    repository.AgentRegistryRepository
	PythonExecutor       pythonexec.Executor
}

func NewServiceContext(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker) *ServiceContext {
	return NewServiceContextWithAuth(repo, accountTypeChecker, config.DefaultJWTAuthConfig())
}

func NewServiceContextWithAuth(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth config.JWTAuthConfig) *ServiceContext {
	return NewServiceContextWithAuthAndPythonExecutor(repo, accountTypeChecker, auth, pythonexec.NewDefaultExecutor())
}

func NewServiceContextWithAuthAndPythonExecutor(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth config.JWTAuthConfig, executor pythonexec.Executor) *ServiceContext {
	if executor == nil {
		executor = pythonexec.NewDefaultExecutor()
	}
	return &ServiceContext{
		AuthRuntime:    common.NewAuthRuntime(auth),
		AgentLogic:     logic.NewAgentLogic(repo, accountTypeChecker),
		AgentRepo:      repo,
		PythonExecutor: executor,
	}
}

func (ctx *ServiceContext) ConfigureAgentRegistry(registry repository.AgentRegistryRepository) {
	ctx.ConfigureAgentAssembly(nil, nil, registry)
}

func (ctx *ServiceContext) ConfigureAgentAssembly(accounts repository.AccountRepository, friendships repository.FriendshipRepository, registry repository.AgentRegistryRepository) {
	if ctx == nil {
		return
	}
	ctx.AgentRegistryRepo = registry
	ctx.AgentDefinitionLogic = logic.NewAgentAssemblyLogic(logic.AgentAssemblyDependencies{
		Accounts:    accounts,
		Friendships: friendships,
		Agents:      ctx.AgentRepo,
		Registry:    registry,
	})
}
