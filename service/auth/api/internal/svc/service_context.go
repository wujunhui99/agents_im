// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/service/auth/api/internal/config"
	"github.com/wujunhui99/agents_im/service/auth/rpc/authclient"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrAuthRPCConfigRequired = errors.New("auth rpc client config is required")

type ServiceContext struct {
	Config  config.Config
	AuthRPC authclient.Auth
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.AuthRPC) {
		return nil, ErrAuthRPCConfigRequired
	}
	cli, err := zrpc.NewClient(c.AuthRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:  c,
		AuthRPC: authclient.NewAuth(cli),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
