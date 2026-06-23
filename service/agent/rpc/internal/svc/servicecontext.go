package svc

import (
	"log"
	"os"
	"strings"

	"github.com/zeromicro/go-zero/zrpc"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aihosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/consumer"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/hosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/imadapter"
	orchestrator "github.com/wujunhui99/agents_im/service/agent/rpc/internal/orchestrator"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registry"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/trigger"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/userrpc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
)

// ServiceContext 装配 agent-rpc：gRPC 面（AI 托管开关 CRUD，经 Hosting.AIHostingLogic）
// + agent.trigger.v1 消费 worker（Consumer：judge 终判 → ScheduleTrigger）。AI 回复写回
// 经 imadapter.MsgRPCSender → msg-rpc gRPC SendMessage（D15 step ④）。conversation_ai_hosting
// 数据层已 goctl 化（convhosting，AG-6 ①/D13）；仍 import 的 internal/{logic,repository}
// 是剩余 keystone 例外（agent registry/audit/message 等数据层未 goctl 化，AG-6 ②③… 前）。
type ServiceContext struct {
	Config   config.Config
	Hosting  *aihosting.ServiceContext
	Consumer *consumer.Consumer

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

	msgRPCClient, err := zrpc.NewClient(c.MsgRPC)
	if err != nil {
		log.Fatalf("build msg rpc client: %v", err)
	}
	userRPCClient, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		log.Fatalf("build user rpc client: %v", err)
	}
	userCli := userclient.NewUser(userRPCClient)

	hostingCtx := buildHostingRuntime(c, userCli, imadapter.NewMsgRPCSender(msgRPCClient))

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
		Config:       c,
		Hosting:      hostingCtx,
		Consumer:     triggerConsumer,
		KafkaBrokers: resolveKafkaBrokers(c),
		KafkaGroup:   c.Kafka.Group,
	}
}

// buildHostingRuntime 装配 AI 托管运行时（runtime + request builder + audit + 写回 +
// CHS）。移植自原 msg-rpc newConversationAIHostingRuntime（#341/#463），随 agent 域迁入
// 属主；AI 写回通道改注入 imadapter.MsgRPCSender（经 msg-rpc gRPC，不再本进程直写 PG）。
func buildHostingRuntime(c config.Config, userCli userclient.User, responseSender orchestrator.MessageSender) *aihosting.ServiceContext {
	messageRepo, err := repository.NewMessageRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	// friendshipRepo 提供 agent-create 工具路径的好友写（friendships 表无 avatar，非 #550 blocker）。
	friendshipRepo, err := repository.NewRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	// AccountRepo = Composite：账号读写经 user-rpc（脱 internal avatar 读写），好友写委托 postgres repo。
	accountRepo := userrpc.NewComposite(userCli, friendshipRepo)
	groupsRepo, err := repository.NewGroupsRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	groupsLogic := business.NewGroupsLogic(groupsRepo, nil)
	agentHostingRepo, err := repository.NewAgentConversationHostingRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build agent hosting repository: %v", err)
	}
	agentRepo, err := repository.NewAgentRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build agent repository: %v", err)
	}
	agentRegistryRepo, err := repository.NewAgentRegistryRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build agent registry repository: %v", err)
	}
	// conversation_ai_hosting 数据层已脱 internal/repository，改 agent 自有 goctl model（AG-6 ① / D13）。
	aiHostingStore := convhosting.NewModelStore(appconfig.ResolveDataSource(c.DataSource))
	// 注册表只读路径（runtime tool 解析 + 请求构建）改 agent 自有 goctl model（#605）。
	// 写路径（agent.create keystone）仍用上面的 agentRegistryRepo，待后续 PR 迁出 internal。
	agentRegistryReader := registry.NewStore(appconfig.ResolveDataSource(c.DataSource))
	agentAuditRepo, err := repository.NewAgentAuditRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build agent audit repository: %v", err)
	}
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
	hostingCtx.AgentHostingRepo = agentHostingRepo
	hostingCtx.AIHostingStore = aiHostingStore
	hostingCtx.AgentResolver = orchestrator.NewAgentRepositoryAccountResolver(agentRepo)
	hostingCtx.AccountRepo = accountRepo
	hostingCtx.AgentRepo = agentRepo
	hostingCtx.AIHostingLogic = convhosting.NewConversationAIHostingLogic(aiHostingStore).WithAgentAccountResolver(hostingCtx.AgentResolver)
	hostingCtx.AgentAuditRepo = agentAuditRepo
	hostingCtx.AgentAuditLogic = business.NewAgentAuditLogic(agentAuditRepo)
	hostingCtx.AgentRegistryRepo = agentRegistryRepo
	hostingCtx.AgentRegistryReader = agentRegistryReader
	hostingCtx.PythonExecutor = pythonExecutor
	// AI 写回经 msg-rpc gRPC SendMessage（imadapter），AI 消息走与人类消息相同的 Kafka 链路。
	hostingCtx.AgentResponseSender = responseSender
	if err := aihosting.ConfigureConversationAIHosting(hostingCtx, c.DeepSeek, c.LLMObservability); err != nil {
		log.Fatalf("configure AI conversation hosting: %v", err)
	}
	return hostingCtx
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
