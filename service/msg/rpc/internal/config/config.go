package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// msg-rpc 数据层走 goctl model（Postgres-only），不再支持 memory driver。
	// tracing 用 go-zero 自带 Telemetry（在 RpcServerConf.ServiceConf 内，由 yaml 配置）。
	DataSource string `json:",optional"`

	// UserRPC：AI 托管运行时 agent-create 工具路径的账号读写经属主 user-rpc
	// （gate #550，脱 internal/repository accountRepo 的 avatar string scan/空串写）。
	UserRPC zrpc.RpcClientConf `json:",optional"`

	// MediaRPC：SendMessage 写路径的图片/文件附件校验经属主 media-rpc
	// （#533，脱 internal/mediavalidate 直读 media_objects）。
	MediaRPC zrpc.RpcClientConf `json:",optional"`

	// AI 托管运行时（keystone 例外：随 message-api 退役迁入，SendMessage 后触发 Agent 回复；
	// 待 03-message-pipeline §9 B1 把触发点迁到 msgtransfer 后删除）。
	DeepSeek         appconfig.DeepSeekConfig         `json:",optional"`
	LLMObservability appconfig.LLMObservabilityConfig `json:",optional"`
	PythonExecutor   appconfig.PythonExecutorConfig   `json:",optional"`

	// Kafka 唯一写链路（03 §9 B3b）：见 KafkaConfig。
	Kafka KafkaConfig `json:",optional"`

	// Snowflake 配置 message_id 雪花生成器的机器位（EPIC #527 §0：多副本同毫秒不碰撞）。
	Snowflake SnowflakeConfig `json:",optional"`
}

// SnowflakeConfig 配置 message_id 的 RoutedFlake 生成器。msg HintBits=1（中段最高位单/群区分）。
type SnowflakeConfig struct {
	// MachineBits 是机器号位宽（中段 12 位的低端）；0 时 svc 取默认值。
	MachineBits uint `json:",optional"`
	// MachineID 是本实例机器号。运行期优先用 idgen.ResolveMachineID()（env
	// AGENTS_IM_SNOWFLAKE_MACHINE_ID 或 StatefulSet pod ordinal）；解析不到时回退到本值
	// （默认 0，适用单副本 Deployment）。多副本必须经 env/ordinal 注入唯一机器号。
	MachineID int64 `json:",optional"`
}

// KafkaConfig 是 Kafka 写链路配置（03-message-pipeline §9 B2/B3b）：SendMessage
// 只 publish message.submitted 到 msg.toTransfer.v1（不写 PG、ACK 不带 seq），
// AI 触发经 agent.trigger.v1 consumer 回流。B3b 起旧 PG 同步写已退役：brokers
// 必填（缺失启动失败），MSG_DIRECT_KAFKA=false 显式拒绝；Enabled 字段仅作兼容保留。
type KafkaConfig struct {
	Enabled bool   `json:",optional"`
	Brokers string `json:",optional"`
}
