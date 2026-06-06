// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/config"
	"github.com/wujunhui99/agents_im/service/friends/rpc/friendsclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

var (
	ErrFriendsRPCConfigRequired = errors.New("friends rpc client config is required")
	ErrUserRPCConfigRequired    = errors.New("user rpc client config is required")
)

type ServiceContext struct {
	Config     config.Config
	FriendsRPC friendsclient.Friends
	UserRPC    userclient.User
	DeviceAuth rest.Middleware
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.FriendsRPC) {
		return nil, ErrFriendsRPCConfigRequired
	}
	if !hasRPCClientConfig(c.UserRPC) {
		return nil, ErrUserRPCConfigRequired
	}
	cli, err := zrpc.NewClient(c.FriendsRPC)
	if err != nil {
		return nil, err
	}
	userCli, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:     c,
		FriendsRPC: friendsclient.NewFriends(cli),
		UserRPC:    userclient.NewUser(userCli),
		DeviceAuth: middleware.NewDeviceAuthMiddleware(middleware.NewRedisSessionStore(c.Redis)).Handle,
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
