package svc

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/zrpc"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogic"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aghosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aihosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/consumer"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/hosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/imadapter"
	orchestrator "github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
	runtimetools "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime/tools"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/trigger"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/userrpc"
	friendsclient "github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// ServiceContext 装配 agent-rpc：gRPC 面（agent CRUD + 定义 + 默认助手装配 + AI 托管开关，#606
// 数据层脱 internal/）+ agent.trigger.v1 消费 worker。agent 域数据走自有 goctl（agentlogic.AgentStore /
// registry.Store / convhosting）；跨域账号/好友经 user-rpc/friends-rpc 端口。仍 import 的
// internal/{logic,repository} 是 AI runtime keystone（MessageLogic / messages / agent hosting+audit
// 数据层），待 message 迁移（AG-6③）后清。
type ServiceContext struct {
	Config   config.Config
	Hosting  *aihosting.ServiceContext
	Consumer *consumer.Consumer

	// agent 域业务逻辑（gRPC 面），背靠 agent 自有 goctl 数据层 + 跨域端口。
	AgentLogic       *agentlogic.AgentLogic
	AgentAssembly    *agentlogic.AgentAssemblyLogic
	AgentProvisioner *agentlogic.DefaultAssistantProvisioner

	KafkaBrokers []string
	KafkaGroup   string
}

func NewServiceContext(c config.Config) *ServiceContext {
	if strings.TrimSpace(c.DataSource) == "" {
		log.Fatalf("agent-rpc requires DataSource")
	}
	if !hasRPCClientConfig(c.MsgRPC) {
		log.Fatalf("agent-rpc requires msg rpc client config (MsgRPC) for AI write-back")
	}
	if !hasRPCClientConfig(c.UserRPC) {
		log.Fatalf("agent-rpc requires user rpc client config (UserRPC) for agent-create account access")
	}
	if !hasRPCClientConfig(c.FriendsRPC) {
		log.Fatalf("agent-rpc requires friends rpc client config (FriendsRPC) for agent-create friendship")
	}

	msgRPCClient, err := zrpc.NewClient(c.MsgRPC)
	if err != nil {
		log.Fatalf("build msg rpc client: %v", err)
	}
	userRPCClient, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		log.Fatalf("build user rpc client: %v", err)
	}
	friendsRPCClient, err := zrpc.NewClient(c.FriendsRPC)
	if err != nil {
		log.Fatalf("build friends rpc client: %v", err)
	}

	ds := appconfig.ResolveDataSource(c.DataSource)
	// agent 域自有数据层（goctl）：agents 表 + 注册表（读写）。
	agentStore := agentlogic.NewAgentStore(ds)
	registryStore := registry.NewStore(ds)
	// 跨域端口：账号经 user-rpc、好友经 friends-rpc（单向叶子，不成环）。
	accountPort := userrpc.NewAccountClient(userclient.NewUser(userRPCClient))
	friendPort := userrpc.NewFriendClient(friendsclient.NewFriends(friendsRPCClient))

	agentLogic := agentlogic.NewAgentLogic(agentStore, accountPort)
	assembly := agentlogic.NewAgentAssemblyLogic(agentlogic.AgentAssemblyDependencies{
		Accounts:    accountPort,
		Friendships: friendPort,
		Agents:      agentStore,
		Registry:    registryStore,
	})
	provisioner := agentlogic.NewDefaultAssistantProvisioner(agentStore, registryStore)

	hostingCtx := buildHostingRuntime(c, imadapter.NewMsgRPCSender(msgRPCClient), agentStore, registryStore, assembly)

	// trigger.Judge 终判第 3 步（hosting 查询）直接读 agent 域 conversation_ai_hosting
	// （hosting.Store over agent 自有 convhosting.Store / goctl model；AG-6 ① 已脱 internal）。
	hostingStore, err := hosting.NewStore(hostingCtx.AIHostingStore)
	if err != nil {
		log.Fatalf("build hosting store: %v", err)
	}
	judge, err := trigger.NewJudge(hostingStore)
	if err != nil {
		log.Fatalf("build trigger judge: %v", err)
	}
	triggerConsumer, err := consumer.New(judge, hostingCtx.HostingService)
	if err != nil {
		log.Fatalf("build trigger consumer: %v", err)
	}

	return &ServiceContext{
		Config:           c,
		Hosting:          hostingCtx,
		Consumer:         triggerConsumer,
		AgentLogic:       agentLogic,
		AgentAssembly:    assembly,
		AgentProvisioner: provisioner,
		KafkaBrokers:     resolveKafkaBrokers(c),
		KafkaGroup:       c.Kafka.Group,
	}
}

// buildHostingRuntime 装配 AI 托管运行时（runtime + request builder + audit + 写回 + CHS）。
// agent 域读路径（agents/registry）走自有 goctl store；agent.create 工具处理器由本域 agentlogic
// assembly（goctl + user-rpc/friends-rpc 端口）装配注入。仍用 internal/{logic,repository} 的是 AI
// runtime keystone（messages / 群成员鉴权 / agent hosting+audit 数据层），待 message 迁移清。
func buildHostingRuntime(c config.Config, responseSender orchestrator.MessageSender, agentStore agentlogic.AgentStore, registryStore *registry.Store, assembly *agentlogic.AgentAssemblyLogic) *aihosting.ServiceContext {
	messageRepo, err := repository.NewMessageRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	groupsRepo, err := repository.NewGroupsRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	groupsLogic := business.NewGroupsLogic(groupsRepo, nil)
	// agent_conversation_hosting + agent_trigger_idempotency 数据层已脱 internal/repository，
	// 改 agent 自有 goctl model Store（#670，从 #616 拆出）。
	agentHostingStore := aghosting.NewModelStore(appconfig.ResolveDataSource(c.DataSource))
	// conversation_ai_hosting 数据层已脱 internal/repository，改 agent 自有 goctl model（AG-6 ① / D13）。
	aiHostingStore := convhosting.NewModelStore(appconfig.ResolveDataSource(c.DataSource))
	// agent 审计四表（agent_runs/tool_calls/file_reads/python_execs）数据层已脱 internal/repository，
	// 改 agent 自有 goctl model Store（#616，从 #344/#394 拆出）。
	agentAuditStore := agaudit.NewModelStore(appconfig.ResolveDataSource(c.DataSource))
	var pythonExecutorClient pythonexec.KubernetesSandboxClient
	if c.PythonExecutor.Backend == appconfig.PythonExecutorBackendK8S {
		pythonExecutorClient, err = pythonexec.NewInClusterKubernetesSandboxClient()
		if err != nil {
			log.Fatalf("build python executor kubernetes client: %v", err)
		}
	}
	pythonExecutor, err := pythonexec.NewExecutorFromConfig(c.PythonExecutor, pythonExecutorClient)
	if err != nil {
		log.Fatalf("build python executor: %v", err)
	}

	// 附件校验在 agent 写回路径不可达（AI 回复经 imadapter→msg-rpc，由 msg-rpc 校验）；
	// MessageLogic 在本进程只作运行时数据层（历史读）与被覆盖的 fallback sender，传 nil 用放行 fixture。
	hostingCtx := aihosting.NewServiceContextWithMediaValidator(messageRepo, nil, nil, groupsLogic, appconfig.DefaultJWTAuthConfig())
	hostingCtx.AgentHostingRepo = agentHostingStore
	hostingCtx.AIHostingStore = aiHostingStore
	// AgentResolver / 请求构建器读 agents 表改 agent 自有 goctl AgentStore（#606，脱 internal）。
	hostingCtx.AgentResolver = orchestrator.NewAgentRepositoryAccountResolver(agentStore)
	hostingCtx.AIHostingLogic = convhosting.NewConversationAIHostingLogic(aiHostingStore).WithAgentAccountResolver(hostingCtx.AgentResolver)
	hostingCtx.AgentAudit = agentAuditStore
	// 注册表只读路径（runtime tool 解析 + 请求构建）改 agent 自有 goctl Store（#605/#606）。
	hostingCtx.AgentRegistryReader = registryStore
	// agent.create 工具处理器：agent 自有 agentlogic assembly（goctl + user-rpc/friends-rpc 端口，#606）。
	hostingCtx.AgentCreate = newAgentCreateHandler(assembly)
	hostingCtx.PythonExecutor = pythonExecutor
	// AI 写回经 msg-rpc gRPC SendMessage（imadapter），AI 消息走与人类消息相同的 Kafka 链路。
	hostingCtx.AgentResponseSender = responseSender
	if err := aihosting.ConfigureConversationAIHosting(hostingCtx, c.DeepSeek, c.LLMObservability); err != nil {
		log.Fatalf("configure AI conversation hosting: %v", err)
	}
	return hostingCtx
}

// newAgentCreateHandler 把 agent 域 assembly 的 CreateAgentFromTool 适配成 runtime 工具处理器。
func newAgentCreateHandler(assembly *agentlogic.AgentAssemblyLogic) runtimetools.AgentCreateHandler {
	if assembly == nil {
		return nil
	}
	return runtimetools.AgentCreateHandlerFunc(func(ctx context.Context, req runtimetools.AgentCreateRequest) (runtimetools.AgentCreateResponse, error) {
		created, err := assembly.CreateAgentFromTool(ctx, agentlogic.AgentCreateToolRequest{
			CreatorAgentID:   req.CreatorAgentID,
			RequestingUserID: req.RequestingUserID,
			Identifier:       req.Identifier,
			Name:             req.Name,
			Description:      req.Description,
			SystemPrompt:     req.SystemPrompt,
			ToolNames:        req.ToolNames,
		})
		if err != nil {
			return runtimetools.AgentCreateResponse{}, err
		}
		return runtimetools.AgentCreateResponse{
			AgentID:      created.AgentID,
			AccountID:    created.AccountID,
			Identifier:   created.Identifier,
			Name:         created.Name,
			Description:  created.Description,
			PromptID:     created.PromptID,
			ToolNames:    created.ToolNames,
			FriendUserID: created.FriendUserID,
		}, nil
	})
}

// resolveKafkaBrokers 解析 agent.trigger.v1 的 brokers：env KAFKA_BROKERS 覆盖 yaml。
func resolveKafkaBrokers(c config.Config) []string {
	brokers := appconfig.KafkaBrokerList(firstNonEmpty(
		strings.TrimSpace(os.Getenv("KAFKA_BROKERS")),
		strings.TrimSpace(os.ExpandEnv(c.Kafka.Brokers)),
	))
	if len(brokers) == 0 {
		log.Fatalf("agent-rpc requires Kafka brokers (set Kafka.Brokers or KAFKA_BROKERS)")
	}
	return brokers
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置（target / endpoints / etcd 任一）。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
