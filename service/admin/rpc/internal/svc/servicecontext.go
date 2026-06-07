package svc

import (
	"log"

	msglogic "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

// ServiceContext 持有 admin-rpc 的依赖。
//
// admin-rpc 是 admin 域唯一碰 DB 的服务：
//   - TaskReportModel 是 admin 独占表 task_reports 的 goctl 自有数据层。
//   - Accounts/Friends/Messages/AgentAudits/Feedback 为跨域只读，暂经顶层 internal/repository
//     直读（messages/AI-replay 属 message monolith keystone、agent_audits 无 owning rpc、
//     feedback 由 message monolith 创建），待相关域 rpc 落地后迁移为 BFF 聚合。
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
		Accounts:        accounts,
		Friends:         accounts,
		Messages:        messages,
		AgentAudits:     agentAudits,
		Feedback:        feedback,
	}
}
