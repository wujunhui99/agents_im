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
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}
