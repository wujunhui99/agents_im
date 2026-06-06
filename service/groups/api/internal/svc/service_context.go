// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/config"
	"github.com/wujunhui99/agents_im/service/groups/rpc/groupsclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

var (
	ErrGroupsRPCConfigRequired = errors.New("groups rpc client config is required")
	ErrUserRPCConfigRequired   = errors.New("user rpc client config is required")
)

type ServiceContext struct {
	Config     config.Config
	GroupsRPC  groupsclient.Groups
	UserRPC    userclient.User
	DeviceAuth rest.Middleware
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.GroupsRPC) {
		return nil, ErrGroupsRPCConfigRequired
	}
	if !hasRPCClientConfig(c.UserRPC) {
		return nil, ErrUserRPCConfigRequired
	}
	// zrpc 客户端内置 otel tracing 拦截器（go-zero 自带 Telemetry），无需额外注入。
	groupsCli, err := zrpc.NewClient(c.GroupsRPC)
	if err != nil {
		return nil, err
	}
	userCli, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:     c,
		GroupsRPC:  groupsclient.NewGroups(groupsCli),
		UserRPC:    userclient.NewUser(userCli),
		DeviceAuth: middleware.NewDeviceAuthMiddleware(middleware.NewRedisSessionStore(c.Redis)).Handle,
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
