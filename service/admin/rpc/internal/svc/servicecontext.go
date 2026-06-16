package svc

import (
	"log"

	msglogic "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/userrpc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext 持有 admin-rpc 的依赖。
//
// admin-rpc 是 admin 域唯一碰 DB 的服务：
//   - TaskReportModel 是 admin 独占表 task_reports 的 goctl 自有数据层。
//   - Accounts 跨域账号只读经属主 user-rpc（gate #550，脱 internal/repository accountRepo 的
//     avatar string scan）；Friends/Messages/AgentAudits/Feedback 仍跨域只读 internal/repository
//     （friendships 无 avatar 非 #550 blocker；messages/AI-replay 属 message monolith keystone、
//     agent_audits 无 owning rpc、feedback 由 message monolith 创建），待相关域 rpc 落地后迁移。
//   - MessageCreatedHook 供 AI-replay 触发 agent；独立 admin 二进制中本就不接线（nil/休眠），
//     admin-rpc 沿用同样行为，无功能回归。
type ServiceContext struct {
	Config config.Config

	TaskReportModel model.TaskReportsModel

	Accounts    repository.AdminAccountRepository
	Friends     repository.FriendshipRepository
	Messages    repository.AdminMessageRepository
	AgentAudits repository.AdminAgentAuditRepository
	Feedback    repository.FeedbackRepository

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
	accountsViaUserRPC := userrpc.NewAdminAccountClient(userclient.NewUser(userRPCClient))

	// friendships 跨域只读（无 avatar，非 #550 blocker）仍走 internal/repository。
	accounts, err := repository.NewPostgresRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build account repository: %v", err)
	}
	messages, err := repository.NewPostgresMessageRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build message repository: %v", err)
	}
	agentAudits, err := repository.NewPostgresAgentAuditRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build agent audit repository: %v", err)
	}
	feedback, err := repository.NewPostgresFeedbackRepository(c.DataSource)
	if err != nil {
		log.Fatalf("build feedback repository: %v", err)
	}

	return &ServiceContext{
		Config:          c,
		TaskReportModel: model.NewTaskReportsModel(conn),
		Accounts:        accountsViaUserRPC,
		Friends:         accounts,
		Messages:        messages,
		AgentAudits:     agentAudits,
		Feedback:        feedback,
	}
}

// hasRPCClientConfig 判断 zrpc 客户端是否已配置(target / endpoints / etcd 任一)。
func hasRPCClientConfig(conf zrpc.RpcClientConf) bool {
	return conf.Target != "" || len(conf.Endpoints) > 0 || (len(conf.Etcd.Hosts) > 0 && conf.Etcd.Key != "")
}
