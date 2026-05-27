package config

import appconfig "github.com/wujunhui99/agents_im/internal/config"

type Config = appconfig.APIConfig

func Load(path string) (Config, error) {
	return appconfig.LoadAPIConfig(path)
}
