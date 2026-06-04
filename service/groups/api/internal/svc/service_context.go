// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/config"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/zrpc"
)

var (
	ErrGroupsRPCConfigRequired = errors.New("groups rpc client config is required")
	ErrUserRPCConfigRequired   = errors.New("user rpc client config is required")
)

type ServiceContext struct {
	Config    config.Config
	GroupsRPC groupsclient.Groups
	UserRPC   userclient.User
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.GroupsRPC) {
		return nil, ErrGroupsRPCConfigRequired
	}
	if !hasRPCClientConfig(c.UserRPC) {
		return nil, ErrUserRPCConfigRequired
	}
	groupsCli, err := zrpc.NewClient(c.GroupsRPC, zrpc.WithUnaryClientInterceptor(observability.GRPCUnaryClientInterceptor()))
	if err != nil {
		return nil, err
	}
	userCli, err := zrpc.NewClient(c.UserRPC, zrpc.WithUnaryClientInterceptor(observability.GRPCUnaryClientInterceptor()))
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:    c,
		GroupsRPC: groupsclient.NewGroups(groupsCli),
		UserRPC:   userclient.NewUser(userCli),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
