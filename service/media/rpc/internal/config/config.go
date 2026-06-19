package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// media-rpc 已转 Postgres-only：media_objects 数据层走 goctl model，不再支持 memory driver。
	DataSource    string                        `json:",optional"`
	ObjectStorage appconfig.ObjectStorageConfig `json:",optional"`
	// Snowflake 配置 media_id 雪花生成器的机器位（EPIC #527 §1：多副本同毫秒不碰撞）。
	Snowflake SnowflakeConfig `json:",optional"`
	// 下载授权（EPIC #527 §4）的跨域编排在 media-api(BFF)：media-rpc 不持跨域 rpc 客户端、保持叶子。
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}

// SnowflakeConfig 配置 media_id 的 RoutedFlake 生成器。media HintBits=0（无单/群语义）。
type SnowflakeConfig struct {
	// MachineBits 是机器号位宽（中段 12 位的低端）；0 时 svc 取默认值。
	MachineBits uint `json:",optional"`
	// MachineID 是本实例机器号。运行期优先用 idgen.ResolveMachineID()（env
	// AGENTS_IM_SNOWFLAKE_MACHINE_ID 或 StatefulSet pod ordinal）；解析不到时回退到本值
	// （默认 0，适用单副本 Deployment）。多副本必须经 env/ordinal 注入唯一机器号。
	MachineID int64 `json:",optional"`
}
