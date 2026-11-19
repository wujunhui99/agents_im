// Package config 定义 resource-rpc 的配置：一个进程同时承载 media（对象存储生命周期）
// 与 mail（外部邮件/供应商）两个能力域，共享同一个 gRPC 端口与 Telemetry。
package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	mailprovider "github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/provider"
	mediaimplconfig "github.com/wujunhui99/agents_im/service/resource/rpc/internal/media/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf

	// media 能力：media_objects 数据层 + 对象存储（RustFS/S3 兼容）。
	DataSource    string                          `json:",optional"`
	ObjectStorage appconfig.ObjectStorageConfig   `json:",optional"`
	Snowflake     mediaimplconfig.SnowflakeConfig `json:",optional"`

	// mail 能力：外部邮件供应商（腾讯云 SES）。
	TencentSES mailprovider.TencentSESConfig
}
