package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// media-rpc 已转 Postgres-only：media_objects 数据层走 goctl model，不再支持 memory driver。
	DataSource    string                        `json:",optional"`
	ObjectStorage appconfig.ObjectStorageConfig `json:",optional"`
	Tracing       observability.TracingConfig   `json:",optional"`
}
