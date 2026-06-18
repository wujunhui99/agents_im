package svc

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/internal/agentim"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/pythonexec"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/userrpc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// message_id 雪花中段（12 位）布局：msg HintBits=1，最高位（bit 21）单聊=1/群聊=0（100… vs 000…）；
// 机器号靠右（低 10 位，默认 1024 实例），bit 20 为保留间隙。扩副本调 Snowflake.MachineBits
// （机器号靠右收缩、不挪位）。见 pkg/idgen/routedflake.go 与 EPIC #527 §0。
const (
	msgHintBits           = 1
	msgHintSingle         = 1 // 单聊 hint（中段最高位置 1）
	msgHintGroup          = 0 // 群聊 hint（中段最高位置 0）
	defaultMsgMachineBits = 10
)

// MsgHintForChatType 返回 message_id 中段最高位的路由 hint：单聊=1，群聊=0（EPIC #527 §0，
// 供 media 等下游无需查库即可判出单/群）。
func MsgHintForChatType(chatType string) int64 {
	if chatType == model.ChatTypeSingle {
		return msgHintSingle
	}
	return msgHintGroup
}

type ServiceContext struct {
	Config config.Config

	// 消息域自有数据层（goctl model，脱 internal/repository）。
	Messages model.MessagesModel
	Threads  model.ConversationThreadsModel
	States   model.UserConversationStatesModel

	// MsgIDGen 发 message_id 雪花 bigint（EPIC #527 §0：HintBits=1 区分单/群，含机器位防同毫秒碰撞）。
	MsgIDGen *idgen.RoutedFlake

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

	// Kafka 写路径（03 §9 B2/B3b）：SendMessage 只 publish msg.toTransfer.v1，
	// AI 写回也经本进程 SendMessage（防 PG/Redis 双 seq 分裂），AI 触发由
	// agent.trigger.v1 consumer（msg.go 启动）回流到 AgentHook。
	// B3b 起旧 PG 同步写路径已退役，Kafka 是唯一写链路（缺配置启动失败）。
	KafkaBrokers []string
	Producer     EventPublisher

	// agentSender 是 Kafka 模式下 AI 写回的晚绑定承载体（见 agent_sender.go）。
	agentSender *kafkaModeSender
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	msgIDGen, err := newMsgIDGenerator(c.Snowflake)
	if err != nil {
		log.Fatalf("build message id generator: %v", err)
	}

	groupsRepo, err := repository.NewGroupsRepositoryForStorage(appconfig.StorageDriverPostgres, c.DataSource)
	if err != nil {
		log.Fatalf("build groups repository: %v", err)
	}
	groupsLogic := business.NewGroupsLogic(groupsRepo, nil)

	// 图片/文件附件校验经属主 media-rpc（#533，脱 internal/mediavalidate 直读 media_objects）。
	if !hasRPCClientConfig(c.MediaRPC) {
		log.Fatalf("msg-rpc requires media rpc client config (MediaRPC)")
	}
	mediaRPCClient, err := zrpc.NewClient(c.MediaRPC)
	if err != nil {
		log.Fatalf("build media rpc client: %v", err)
	}
	mediaValidator := newMediaRPCMessageValidator(mediaclient.NewMedia(mediaRPCClient))

	// 账号读写经属主 user-rpc（gate #550，脱 internal/repository accountRepo 的 avatar string scan/空串写）。
	if !hasRPCClientConfig(c.UserRPC) {
		log.Fatalf("msg-rpc requires user rpc client config (UserRPC)")
	}
	userRPCClient, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		log.Fatalf("build user rpc client: %v", err)
	}
	userCli := userclient.NewUser(userRPCClient)

	kafkaBrokers := resolveKafkaBrokers(c)
	producer, err := messaging.NewKafkaProducer(kafkaBrokers)
	if err != nil {
		log.Fatalf("build kafka producer: %v", err)
	}
	// AI 写回经本进程 SendMessage（晚绑定 svcCtx），防 PG/Redis 双 seq 分裂。
	senderOverride := &kafkaModeSender{}
	hosting := newConversationAIHostingRuntime(c, mediaValidator, groupsLogic, senderOverride, userCli)

	svcCtx := &ServiceContext{
		Config:       c,
		Messages:     model.NewMessagesModel(conn),
		MsgIDGen:     msgIDGen,
		Threads:      model.NewConversationThreadsModel(conn),
		States:       model.NewUserConversationStatesModel(conn),
		Groups:       groupsLogic,
		Media:        mediaValidator,
		AgentHook:    hosting.AgentMessageHook,
		AIHosting:    hosting.AIHostingLogic,
		KafkaBrokers: kafkaBrokers,
		Producer:     producer,
		agentSender:  senderOverride,
	}
	return svcCtx
}

// newMsgIDGenerator 构造 message_id 的 RoutedFlake（HintBits=1，单/群区分位）。机器号优先用
// idgen.ResolveMachineID()（env AGENTS_IM_SNOWFLAKE_MACHINE_ID 或 StatefulSet pod ordinal）；解析不到时
// 回退到配置值（默认 0，适用单副本）。多副本部署须经 env/ordinal 注入唯一机器号，否则同毫秒碰撞。
func newMsgIDGenerator(cfg config.SnowflakeConfig) (*idgen.RoutedFlake, error) {
	machineBits := cfg.MachineBits
	if machineBits == 0 {
		machineBits = defaultMsgMachineBits
	}
	machineID := cfg.MachineID
	if resolved, err := idgen.ResolveMachineID(); err == nil {
		machineID = resolved
	}
	return idgen.NewRoutedFlake(idgen.RoutedFlakeConfig{
		HintBits:    msgHintBits,
		MachineBits: machineBits,
		MachineID:   machineID,
	})
}

// resolveKafkaBrokers 解析 Kafka brokers。B3b 起 Kafka 是唯一写链路：
// 旧 MSG_DIRECT_KAFKA 回滚开关随 PG 同步写路径一并退役，显式关闭=启动失败（失败优先）。
func resolveKafkaBrokers(c config.Config) []string {
	if value := strings.TrimSpace(os.Getenv("MSG_DIRECT_KAFKA")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			log.Fatalf("invalid MSG_DIRECT_KAFKA value %q: %v", value, err)
		}
		if !parsed {
			log.Fatalf("MSG_DIRECT_KAFKA=false is no longer supported: non-Kafka write path retired (03 §9 B3b)")
		}
	}
	brokers := appconfig.KafkaBrokerList(firstNonEmpty(strings.TrimSpace(os.ExpandEnv(c.Kafka.Brokers)), os.Getenv("KAFKA_BROKERS")))
	if len(brokers) == 0 {
		log.Fatalf("kafka brokers are required: non-Kafka write path retired (03 §9 B3b)")
	}
	return brokers
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置(target / endpoints / etcd 任一)。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}

// newConversationAIHostingRuntime 移植自 service/message-api/main.go 的 AI 托管接线：
// 构造 internal messagesvc.ServiceContext（MessageLogic 仅作 Agent 回复写回通道，写同一批表 +
// outbox，与 msg-rpc goctl 数据层共存）并 ConfigureConversationAIHosting。
func newConversationAIHostingRuntime(c config.Config, mediaValidator business.MessageMediaValidator, groupsLogic *business.GroupsLogic, senderOverride agentim.MessageSender, userCli userclient.User) *messagesvc.ServiceContext {
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

	messageContext := messagesvc.NewServiceContextWithMediaValidator(messageRepo, mediaValidator, nil, groupsLogic, appconfig.DefaultJWTAuthConfig())
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
