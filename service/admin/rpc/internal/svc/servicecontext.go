package svc

import (
	"log"

	msglogic "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/feedbackstore"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agentclient"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext 持有 admin-rpc 的依赖。
//
// admin-rpc 是 admin 域唯一碰 DB 的服务：
//   - TaskReportModel 是 admin 独占表 task_reports 的 goctl 自有数据层。
//   - UserRPC 跨域账号只读直调属主 user-rpc（gate #550，脱 internal/repository accountRepo 的
//     avatar string scan）；logic 直接用 user-rpc client，错误 inline rpcerror，不再经
//     repository.AdminAccountRepository adapter 与 model.User 过渡态。
//     Friends/Messages 仍跨域只读 internal/repository（friendships 无 avatar 非 #550 blocker；
//     messages/AI-replay 属 message monolith keystone），待相关域 rpc 落地后迁移。
//   - Feedback 是 admin 自有数据层（feedbackstore goctl model 背靠 feedback 表，#678 脱 internal）。
//   - MessageCreatedHook 供 AI-replay 触发 agent；独立 admin 二进制中本就不接线（nil/休眠），
//     admin-rpc 沿用同样行为，无功能回归。
type ServiceContext struct {
	Config config.Config

	TaskReportModel model.TaskReportsModel

	UserRPC  userclient.User
	Friends  repository.FriendshipRepository
	Messages repository.AdminMessageRepository
	// AgentRPC：agent 审计 traces/dashboard 只读经属主 agent-rpc gRPC（#616，脱 internal/repository agent_audit）。
	AgentRPC agentclient.Agent
	Feedback feedbackstore.Store

	MessageCreatedHook msglogic.MessageCreatedHook
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := postgres.New(c.DataSource)

	// 跨域账号只读经属主 user-rpc（gate #550，脱 internal/repository accountRepo 的 avatar string scan）。
	if !hasRPCClientConfig(c.UserRPC) {
		log.Fatalf("admin-rpc requires user rpc client config (UserRPC)")
	}
	userRPCClient, err := zrpc.NewClient(c.UserRPC)
	if err != nil {
		log.Fatalf("build user rpc client: %v", err)
	}
	userRPC := userclient.NewUser(userRPCClient)

	// agent 审计 traces/dashboard 只读经属主 agent-rpc gRPC（#616，脱 internal/repository agent_audit 直读）。
	if !hasRPCClientConfig(c.AgentRPC) {
		log.Fatalf("admin-rpc requires agent rpc client config (AgentRPC)")
	}
	agentRPCClient, err := zrpc.NewClient(c.AgentRPC)
	if err != nil {
		log.Fatalf("build agent rpc client: %v", err)
	}
	agentRPC := agentclient.NewAgent(agentRPCClient)

	// friendships 跨域只读（无 avatar，非 #550 blocker）仍走 internal/repository。
	accounts, err := repository.NewPostgresRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	messages, err := repository.NewPostgresMessageRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	return &ServiceContext{
		Config:          c,
		TaskReportModel: model.NewTaskReportsModel(conn),
		UserRPC:         userRPC,
		Friends:         accounts,
		Messages:        messages,
		AgentRPC:        agentRPC,
		Feedback:        feedbackstore.NewModelStore(c.DataSource),
	}
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置(target / endpoints / etcd 任一)。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
