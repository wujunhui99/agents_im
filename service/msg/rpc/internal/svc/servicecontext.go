package svc

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/idgen"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/groupsrpc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
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

	// 跨域鉴权：群成员解析经属主 groups-rpc（#617）、附件校验经属主 media-rpc（#533）。
	// 均为单向叶子调用（msg-rpc → groups/media-rpc），不成环。
	Groups groupsrpc.GroupMemberLister
	Media  MessageMediaValidator

	// Kafka 写路径（03 §9 B2/B3b）：SendMessage 只 publish msg.toTransfer.v1。
	// B3b 起旧 PG 同步写路径已退役，Kafka 是唯一写链路（缺配置启动失败）。
	// AI 托管已整体迁出至属主 agent-rpc（#340，D15 step ④）：msg-rpc 不再跑 agent
	// runtime、不消费 agent.trigger.v1、不持 AI 托管开关——AI 消息经 agent-rpc 的
	// imadapter 以普通 SendMessage gRPC 写回，走与人类消息完全相同的本写路径。
	KafkaBrokers []string
	Producer     EventPublisher
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	msgIDGen, err := newMsgIDGenerator(c.Snowflake)
	if err != nil {
		log.Fatalf("build message id generator: %v", err)
	}

	// 群成员鉴权经属主 groups-rpc ListMembers（#617，脱 internal GroupsLogic 直读 groups 表；
	// 单向叶子调用，不成环）。
	if !hasRPCClientConfig(c.GroupsRPC) {
		log.Fatalf("msg-rpc requires groups rpc client config (GroupsRPC)")
	}
	groupsRPCClient, err := zrpc.NewClient(c.GroupsRPC)
	if err != nil {
		log.Fatalf("build groups rpc client: %v", err)
	}
	groupsClient := groupsrpc.NewClient(groupsclient.NewGroups(groupsRPCClient))

	// 图片/文件附件校验经属主 media-rpc（#533，脱 internal/mediavalidate 直读 media_objects）。
	if !hasRPCClientConfig(c.MediaRPC) {
		log.Fatalf("msg-rpc requires media rpc client config (MediaRPC)")
	}
	mediaRPCClient, err := zrpc.NewClient(c.MediaRPC)
	if err != nil {
		log.Fatalf("build media rpc client: %v", err)
	}
	mediaValidator := newMediaRPCMessageValidator(mediaclient.NewMedia(mediaRPCClient))

	kafkaBrokers := resolveKafkaBrokers(c)
	producer, err := messaging.NewKafkaProducer(kafkaBrokers)
	if err != nil {
		log.Fatalf("build kafka producer: %v", err)
	}

	svcCtx := &ServiceContext{
		Config:       c,
		Messages:     model.NewMessagesModel(conn),
		MsgIDGen:     msgIDGen,
		Threads:      model.NewConversationThreadsModel(conn),
		States:       model.NewUserConversationStatesModel(conn),
		Groups:       groupsClient,
		Media:        mediaValidator,
		KafkaBrokers: kafkaBrokers,
		Producer:     producer,
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

// EventPublisher 是 SendMessage Kafka 写路径需要的最小 producer 面
// （生产实现 messaging.KafkaProducer；测试注入 fake）。
type EventPublisher interface {
	PublishEvent(ctx context.Context, topic string, event messaging.MessageEvent) error
}
