package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
	StorageDriver string                `json:",optional"`
	DataSource    string                `json:",optional"`
	Redis         appconfig.RedisConfig `json:",optional"`
}
