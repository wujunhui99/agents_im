package config

import (
	mailprovider "github.com/wujunhui99/agents_im/service/resource/rpc/internal/mail/provider"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	TencentSES mailprovider.TencentSESConfig
	// tracing 用 go-zero 自带 Telemetry（ServiceConf 内，由 yaml 配置），不再用 pkg/observability。
}
