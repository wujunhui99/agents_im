package config

import (
	commonconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	TokenAuth      commonconfig.JWTAuthConfig
	AdminBootstrap commonconfig.AdminBootstrapConfig `json:",optional"`
	StorageDriver  string                            `json:",default=memory,options=memory|postgres|postgresql"`
	DataSource     string                            `json:",optional"`
	Tracing        observability.TracingConfig       `json:",optional"`
	MailRPC        zrpc.RpcClientConf                `json:",optional"`
}
