package svc

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/internal/agentim"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/mediavalidate"
	"github.com/wujunhui99/agents_im/internal/repository"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

type ServiceContext struct {
	Config config.Config

	// 消息域自有数据层（goctl model，脱 internal/repository）。
	Messages model.MessagesModel
	Threads  model.ConversationThreadsModel
	States   model.UserConversationStatesModel
	Outbox   model.MessageOutboxModel

	// 跨域 keystone 例外：SendMessage 写路径需要 inline 鉴权（群成员解析 + 媒体校验），
	// 无法干净 BFF 化，暂依赖 internal（待 groups/media 完全 BFF/rpc 化后删）。
	Groups business.GroupMemberLister
	Media  business.MessageMediaValidator

	// AI 托管（keystone 例外：随 message-api 退役迁入，待 03-message-pipeline §9 B1 把
	// 触发点迁到 msgtransfer / agent 域 rpc 落地后删除）。
	// AgentHook 在 SendMessage 持久化后触发 Agent 回复（语义对齐原 message-api 进程内
	// MessageLogic.SetMessageCreatedHook）；AIHosting 服务 Get/UpdateConversationAIHosting RPC。
	AgentHook business.MessageCreatedHook
	AIHosting *business.ConversationAIHostingLogic

	// Kafka 写路径（03 §9 B2，flag MSG_DIRECT_KAFKA）：on 时 SendMessage 只 publish
	// msg.toTransfer.v1，AI 写回也经本进程 SendMessage（防 PG/Redis 双 seq 分裂），
	// AI 触发由 agent.trigger.v1 consumer（msg.go 启动）回流到 AgentHook。
	KafkaEnabled bool
	KafkaBrokers []string
	Producer     EventPublisher

	// agentSender 是 Kafka 模式下 AI 写回的晚绑定承载体（见 agent_sender.go）。
	agentSender *kafkaModeSender
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	groupsRepo, err := repository.NewGroupsRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	mediaRepo, err := repository.NewMediaRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build media repository: %v", err)
	}
	groupsLogic := business.NewGroupsLogic(groupsRepo, nil)

	kafkaEnabled, kafkaBrokers := resolveKafkaFlag(c)
	var producer *messaging.KafkaProducer
	var senderOverride *kafkaModeSender
	if kafkaEnabled {
		producer, err = messaging.NewKafkaProducer(kafkaBrokers)
		if err != nil {
			log.Fatalf("build kafka producer (MSG_DIRECT_KAFKA on): %v", err)
		}
		// AI 写回经本进程 SendMessage（晚绑定 svcCtx），防 PG/Redis 双 seq 分裂。
		senderOverride = &kafkaModeSender{}
	}

	// 避免 typed-nil interface：仅启用时才把 override 传给 hosting 接线。
	var senderIface agentim.MessageSender
	if senderOverride != nil {
		senderIface = senderOverride
	}
	hosting := newConversationAIHostingRuntime(c, mediaRepo, groupsLogic, senderIface)

	svcCtx := &ServiceContext{
		Config:       c,
		Messages:     model.NewMessagesModel(conn),
		Threads:      model.NewConversationThreadsModel(conn),
		States:       model.NewUserConversationStatesModel(conn),
		Outbox:       model.NewMessageOutboxModel(conn),
		Groups:       groupsLogic,
		Media:        mediavalidate.NewMessageValidator(mediaRepo),
		AgentHook:    hosting.AgentMessageHook,
		AIHosting:    hosting.AIHostingLogic,
		KafkaEnabled: kafkaEnabled,
		KafkaBrokers: kafkaBrokers,
		agentSender:  senderOverride,
	}
	// 避免 typed-nil interface（disabled 时 Producer 必须是真 nil）。
	if producer != nil {
		svcCtx.Producer = producer
	}
	return svcCtx
}

// resolveKafkaFlag 解析 MSG_DIRECT_KAFKA 开关：环境变量优先于 yaml（秒级回滚开关）。
func resolveKafkaFlag(c config.Config) (bool, []string) {
	enabled := c.Kafka.Enabled
	if value := strings.TrimSpace(os.Getenv("MSG_DIRECT_KAFKA")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			log.Fatalf("invalid MSG_DIRECT_KAFKA value %q: %v", value, err)
		}
		enabled = parsed
	}
	brokers := appconfig.KafkaBrokerList(firstNonEmpty(strings.TrimSpace(os.ExpandEnv(c.Kafka.Brokers)), os.Getenv("KAFKA_BROKERS")))
	if enabled && len(brokers) == 0 {
		log.Fatalf("MSG_DIRECT_KAFKA is on but no kafka brokers configured")
	}
	return enabled, brokers
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// newConversationAIHostingRuntime 移植自 service/message-api/main.go 的 AI 托管接线：
// 构造 internal messagesvc.ServiceContext（MessageLogic 仅作 Agent 回复写回通道，写同一批表 +
// outbox，与 msg-rpc goctl 数据层共存）并 ConfigureConversationAIHosting。
func newConversationAIHostingRuntime(c config.Config, mediaRepo repository.MediaRepository, groupsLogic *business.GroupsLogic, senderOverride agentim.MessageSender) *messagesvc.ServiceContext {
	messageRepo, err := repository.NewMessageRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	accountRepo, err := repository.NewRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
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
	aiHostingRepo, err := repository.NewConversationAIHostingRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build AI hosting repository: %v", err)
	}
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

	messageContext := messagesvc.NewServiceContextWithMedia(messageRepo, mediaRepo, nil, groupsLogic, appconfig.DefaultJWTAuthConfig())
	messageContext.AgentHostingRepo = agentHostingRepo
	messageContext.AIHostingRepo = aiHostingRepo
	messageContext.AgentResolver = agentim.NewAgentRepositoryAccountResolver(agentRepo)
	messageContext.AccountRepo = accountRepo
	messageContext.AgentRepo = agentRepo
	messageContext.AIHostingLogic = business.NewConversationAIHostingLogic(aiHostingRepo).WithAgentAccountResolver(messageContext.AgentResolver)
	messageContext.AgentAuditRepo = agentAuditRepo
	messageContext.AgentAuditLogic = business.NewAgentAuditLogic(agentAuditRepo)
	messageContext.AgentRegistryRepo = agentRegistryRepo
	messageContext.PythonExecutor = pythonExecutor
	// Kafka 模式：AI 写回不直写 PG（MessageLogic），改经 msg-rpc SendMessage（03 §9 B2）。
	messageContext.AgentResponseSender = senderOverride
	if err := messagesvc.ConfigureConversationAIHosting(messageContext, c.DeepSeek, c.LLMObservability); err != nil {
		log.Fatalf("configure AI conversation hosting: %v", err)
	}
	return messageContext
}
