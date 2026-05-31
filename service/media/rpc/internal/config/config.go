package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	StorageDriver string                        `json:",default=memory,options=memory|postgres|postgresql"`
	DataSource    string                        `json:",optional"`
	ObjectStorage appconfig.ObjectStorageConfig `json:",optional"`
	Tracing       observability.TracingConfig   `json:",optional"`
}
