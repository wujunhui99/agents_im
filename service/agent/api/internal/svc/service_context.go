package svc

import (
	"errors"

	"github.com/wujunhui99/agents_im/internal/servicecontext/common"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/middleware"
	apiconfig "github.com/wujunhui99/agents_im/service/agent/api/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agentclient"
	"github.com/zeromicro/go-zero/zrpc"
)

// ErrAgentRPCConfigRequired 在 agent-api 缺 agent-rpc 客户端配置时返回：#606 起 agent-api 转纯
// BFF，agent CRUD / 定义经属主 agent-rpc gRPC，不再 in-process logic / 直连 DB；配置缺失须显式失败。
var ErrAgentRPCConfigRequired = errors.New("agent-api requires agent rpc client config")

// ServiceContext 是 agent-api（纯 BFF，#606）的运行时上下文：只持有鉴权运行时 + agent-rpc 客户端，
// 不再持有 in-process 业务逻辑或数据层句柄（agent 域真相在 agent-rpc）。
type ServiceContext struct {
	common.AuthRuntime
	Config   apiconfig.Config
	AgentRPC agentclient.Agent
}

func NewServiceContextFromConfig(c apiconfig.Config) (*ServiceContext, error) {
	if !hasRPCClientConfig(c.AgentRPC) {
		return nil, ErrAgentRPCConfigRequired
	}
	agentRPCClient, err := zrpc.NewClient(c.AgentRPC)
	if err != nil {
		return nil, err
	}
	serviceContext := &ServiceContext{
		AuthRuntime: common.NewAuthRuntime(c.Auth),
		Config:      c,
		AgentRPC:    agentclient.NewAgent(agentRPCClient),
	}
	serviceContext.Sessions = middleware.NewRedisSessionStore(c.Redis)
	return serviceContext, nil
}

// NewServiceContextWithAuth 供单测构造（注入 fake agent-rpc 客户端 + 鉴权配置）。
func NewServiceContextWithAuth(agentRPC agentclient.Agent, auth config.JWTAuthConfig) *ServiceContext {
	return &ServiceContext{
		AuthRuntime: common.NewAuthRuntime(auth),
		Config:      apiconfig.Config{Auth: auth},
		AgentRPC:    agentRPC,
	}
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置（target / endpoints / etcd 任一）。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
