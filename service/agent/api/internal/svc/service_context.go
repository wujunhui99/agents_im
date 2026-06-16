package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	apiconfig "github.com/wujunhui99/agents_im/service/agent/api/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/userrpc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/zrpc"
)

// ErrUserRPCConfigRequired 在 agent-api 缺 user-rpc 客户端配置时返回：账号资料读经属主
// user-rpc(gate #550，脱 internal/repository accountRepo)，配置缺失须显式失败而非静默回退。
var ErrUserRPCConfigRequired = errors.New("agent-api requires user rpc client config")

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
	// 账号资料读经属主 user-rpc(gate #550，脱 internal/repository accountRepo 的 avatar string scan)。
	if !hasRPCClientConfig(c.UserRPC) {
		return nil, ErrUserRPCConfigRequired
	}
	userRPCClient, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		return nil, err
	}
	accounts := userrpc.NewAccountClient(userclient.NewUser(userRPCClient))

	// agents/agent_prompts 等 agent 域数据层(无 avatar)仍走 internal/repository，随 agent 域迁移再脱。
	agentRepo, err := repository.NewAgentRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, err
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(c.StorageDriver, c.DataSource)
	if err != nil {
		return nil, err
	}
	userLogic := logic.NewUserLogic(accounts)

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
	// 好友写(CreateAgentFromTool 路径)在 agent-api HTTP 不可达，不注入 friendships；
	// 若误触发，AgentAssemblyLogic.ensureCreateToolConfigured 会因 friendships==nil 显式失败。
	serviceContext.ConfigureAgentAssembly(accounts, nil, agentRegistryRepo)

	serviceContext.Sessions = middleware.NewRedisSessionStore(c.Redis)

	return serviceContext, nil
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置(target / endpoints / etcd 任一)。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
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
