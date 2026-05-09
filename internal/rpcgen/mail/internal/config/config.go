package config

import (
	"github.com/wujunhui99/agents_im/internal/mail"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	TencentSES mail.TencentSESConfig
}
