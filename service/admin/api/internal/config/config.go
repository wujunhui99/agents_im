package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	Tracing       observability.TracingConfig `json:",optional"`
	StorageDriver string                      `json:",optional"`
	DataSource    string                      `json:",optional"`
	Redis         appconfig.RedisConfig       `json:",optional"`
}
