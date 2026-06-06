package svc

import (
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	apiconfig "github.com/wujunhui99/agents_im/service/agent/api/internal/config"
)

type ServiceContext struct {
	common.AuthRuntime
	Config               apiconfig.Config
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

func NewServiceContextFromConfig(c apiconfig.Config) (*ServiceContext, error) {
	userRepo, err := repository.NewRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, err
	}
	agentRepo, err := repository.NewAgentRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, err
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, err
	}
	userLogic := logic.NewUserLogic(userRepo)

	var pythonExecutorClient pythonexec.KubernetesSandboxClient
	if c.PythonExecutor.Backend == config.PythonExecutorBackendK8S {
		pythonExecutorClient, err = pythonexec.NewInClusterKubernetesSandboxClient()
		if err != nil {
			return nil, err
		}
	}
	pythonExecutor, err := pythonexec.NewExecutorFromConfig(c.PythonExecutor, pythonExecutorClient)
	if err != nil {
		return nil, err
	}

	serviceContext := NewServiceContextWithAuthAndPythonExecutor(
		agentRepo,
		logic.NewUserLogicAccountTypeChecker(userLogic),
		c.Auth,
		pythonExecutor,
	)
	serviceContext.Config = c
	serviceContext.ConfigureAgentAssembly(userRepo, userRepo, agentRegistryRepo)

	serviceContext.Sessions = middleware.NewRedisSessionStore(c.Redis)

	return serviceContext, nil
}

func NewServiceContextWithAuthAndPythonExecutor(repo repository.AgentRepository, accountTypeChecker logic.UserAccountTypeChecker, auth config.JWTAuthConfig, executor pythonexec.Executor) *ServiceContext {
	if executor == nil {
		executor = pythonexec.NewDefaultExecutor()
	}
	return &ServiceContext{
		AuthRuntime:    common.NewAuthRuntime(auth),
		Config:         apiconfig.Config{Auth: auth},
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
