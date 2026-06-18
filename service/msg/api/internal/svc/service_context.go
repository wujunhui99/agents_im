// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/pkg/middleware"
	"github.com/wujunhui99/agents_im/service/admin/rpc/adminclient"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

var (
	ErrMsgRPCConfigRequired = errors.New("msg rpc client config is required")
	// feedback 创建经 admin-rpc（feedback 数据层 owner），BFF 聚合不落本地 DB。
	ErrAdminRPCConfigRequired = errors.New("admin rpc client config is required")
)

type ServiceContext struct {
	Config     config.Config
	MsgRPC     msgclient.Msg
	AdminRPC   adminclient.Admin
	DeviceAuth rest.Middleware
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.MsgRPC) {
		return nil, ErrMsgRPCConfigRequired
	}
	if !hasRPCClientConfig(c.AdminRPC) {
		return nil, ErrAdminRPCConfigRequired
	}
	// zrpc 客户端内置 otel tracing 拦截器（go-zero 自带 Telemetry），无需额外注入。
	msgCli, err := zrpc.NewClient(c.MsgRPC)
	if err != nil {
		return nil, err
	}
	adminCli, err := zrpc.NewClient(c.AdminRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:     c,
		MsgRPC:     msgclient.NewMsg(msgCli),
		AdminRPC:   adminclient.NewAdmin(adminCli),
		DeviceAuth: middleware.NewDeviceAuthMiddleware(middleware.NewRedisSessionStore(c.Redis)).Handle,
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
