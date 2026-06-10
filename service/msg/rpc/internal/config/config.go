package config

import (
	appconfig "github.com/wujunhui99/agents_im/pkg/config"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// msg-rpc 数据层走 goctl model（Postgres-only），不再支持 memory driver。
	// tracing 用 go-zero 自带 Telemetry（在 RpcServerConf.ServiceConf 内，由 yaml 配置）。
	DataSource string `json:",optional"`

	// AI 托管运行时（keystone 例外：随 message-api 退役迁入，SendMessage 后触发 Agent 回复；
	// 待 03-message-pipeline §9 B1 把触发点迁到 msgtransfer 后删除）。
	DeepSeek         appconfig.DeepSeekConfig         `json:",optional"`
	LLMObservability appconfig.LLMObservabilityConfig `json:",optional"`
	PythonExecutor   appconfig.PythonExecutorConfig   `json:",optional"`
}
