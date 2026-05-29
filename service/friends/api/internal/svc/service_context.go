// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/config"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrFriendsRPCConfigRequired = errors.New("friends rpc client config is required")

type ServiceContext struct {
	Config     config.Config
	FriendsRPC friendsclient.Friends
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.FriendsRPC) {
		return nil, ErrFriendsRPCConfigRequired
	}
	cli, err := zrpc.NewClient(c.FriendsRPC, zrpc.WithUnaryClientInterceptor(observability.GRPCUnaryClientInterceptor()))
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:     c,
		FriendsRPC: friendsclient.NewFriends(cli),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
