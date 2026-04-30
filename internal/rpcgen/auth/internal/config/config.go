package config

import (
	commonconfig "github.com/wujunhui99/agents_im/internal/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	JWTAuth       commonconfig.JWTAuthConfig
	StorageDriver string `json:",default=memory,options=memory|postgres"`
	DataSource    string `json:",optional"`
}
