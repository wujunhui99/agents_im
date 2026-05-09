package mailadapter

import (
	"github.com/wujunhui99/agents_im/internal/rpcgen/mail/mailservice"
	"github.com/zeromicro/go-zero/zrpc"
)

func NewOptionalRPCClient(conf zrpc.RpcClientConf) (Client, error) {
	if !HasRPCClientConfig(conf) {
		return nil, nil
	}
	cli, err := zrpc.NewClient(conf)
	if err != nil {
		return nil, err
	}
	return NewRPCClient(mailservice.NewMailService(cli)), nil
}

func HasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
