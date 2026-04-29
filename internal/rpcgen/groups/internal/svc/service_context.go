package svc

import "github.com/wujunhui99/agents_im/internal/rpcgen/groups/internal/config"

type ServiceContext struct {
	Config config.Config
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
