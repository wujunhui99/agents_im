// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/rpc/authservice"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrAuthRPCConfigRequired = errors.New("auth rpc client config is required")

type ServiceContext struct {
	Config  config.Config
	AuthRPC authservice.AuthService
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.AuthRPC) {
		return nil, ErrAuthRPCConfigRequired
	}
	cli, err := zrpc.NewClient(c.AuthRPC, zrpc.WithUnaryClientInterceptor(observability.GRPCUnaryClientInterceptor()))
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:  c,
		AuthRPC: authservice.NewAuthService(cli),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
