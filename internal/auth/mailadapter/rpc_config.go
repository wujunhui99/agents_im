package mailadapter

import (
	"errors"

	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/service/mail/rpc/mailservice"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrRPCClientConfigRequired = errors.New("mail rpc client config is required")

func NewOptionalRPCClient(conf zrpc.RpcClientConf) (Client, error) {
	if !HasRPCClientConfig(conf) {
		return nil, nil
	}
	cli, err := zrpc.NewClient(conf, zrpc.WithUnaryClientInterceptor(observability.GRPCUnaryClientInterceptor()))
	if err != nil {
		return nil, err
	}
	return NewRPCClient(mailservice.NewMailService(cli)), nil
}

func NewRequiredRPCClient(conf zrpc.RpcClientConf) (Client, error) {
	if !HasRPCClientConfig(conf) {
		return nil, ErrRPCClientConfigRequired
	}
	return NewOptionalRPCClient(conf)
}

func HasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
