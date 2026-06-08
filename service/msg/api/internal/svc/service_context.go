// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/common/middleware"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/config"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrMsgRPCConfigRequired = errors.New("msg rpc client config is required")

type ServiceContext struct {
	Config     config.Config
	MsgRPC     msgclient.Msg
	DeviceAuth rest.Middleware
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.MsgRPC) {
		return nil, ErrMsgRPCConfigRequired
	}
	// zrpc 客户端内置 otel tracing 拦截器（go-zero 自带 Telemetry），无需额外注入。
	msgCli, err := zrpc.NewClient(c.MsgRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:     c,
		MsgRPC:     msgclient.NewMsg(msgCli),
		DeviceAuth: middleware.NewDeviceAuthMiddleware(middleware.NewRedisSessionStore(c.Redis)).Handle,
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
