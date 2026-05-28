package config

import (
	"os"
	"strings"

	commonconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/observability"
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

func (c *Config) ResolveEnvPlaceholders() {
	c.TokenAuth.AccessSecret = os.ExpandEnv(c.TokenAuth.AccessSecret)
	c.AdminBootstrap.Identifier = strings.TrimSpace(os.ExpandEnv(c.AdminBootstrap.Identifier))
	c.AdminBootstrap.Password = os.ExpandEnv(c.AdminBootstrap.Password)
	c.AdminBootstrap.DisplayName = strings.TrimSpace(os.ExpandEnv(c.AdminBootstrap.DisplayName))
	c.StorageDriver = strings.TrimSpace(os.ExpandEnv(c.StorageDriver))
	c.DataSource = os.ExpandEnv(c.DataSource)
	c.Tracing.ServiceName = strings.TrimSpace(os.ExpandEnv(c.Tracing.ServiceName))
	c.Tracing.Environment = strings.TrimSpace(os.ExpandEnv(c.Tracing.Environment))
	c.Tracing.OTLPEndpoint = strings.TrimSpace(os.ExpandEnv(c.Tracing.OTLPEndpoint))
	c.Tracing.Protocol = strings.TrimSpace(os.ExpandEnv(c.Tracing.Protocol))
	c.Tracing.TraceUIBaseURL = strings.TrimSpace(os.ExpandEnv(c.Tracing.TraceUIBaseURL))
}
