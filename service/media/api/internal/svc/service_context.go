package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/service/media/api/internal/config"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/zrpc"
)

var ErrMediaRPCConfigRequired = errors.New("media rpc client config is required")

type ServiceContext struct {
	Config   config.Config
	MediaRPC mediaclient.Media
}

func NewServiceContext(c config.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.MediaRPC) {
		return nil, ErrMediaRPCConfigRequired
	}
	cli, err := zrpc.NewClient(c.MediaRPC)
	if err != nil {
		return nil, err
	}
	return &ServiceContext{
		Config:   c,
		MediaRPC: mediaclient.NewMedia(cli),
	}, nil
}

func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
