package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/agentaudit"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetDashboardLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetDashboardLogic {
	return &GetDashboardLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetDashboard 汇总跨域总量 + 最近 LLM trace + 最近会话状态。
func (l *GetDashboardLogic) GetDashboard(in *admin.DashboardRequest) (*admin.DashboardResponse, error) {
	if l.svcCtx.Accounts == nil || l.svcCtx.Messages == nil || l.svcCtx.AgentAudits == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin repositories are not configured"))
	}
	users, err := l.svcCtx.Accounts.CountAccounts(l.ctx)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	conversations, err := l.svcCtx.Messages.CountConversations(l.ctx)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	messages, err := l.svcCtx.Messages.CountMessages(l.ctx)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	aiRuns, err := l.svcCtx.AgentAudits.CountAgentRuns(l.ctx, "")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	failedRuns, err := l.svcCtx.AgentAudits.CountAgentRuns(l.ctx, string(agentaudit.StatusFailed))
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	limit := normalizeAdminLimit(int(in.GetLimit()), 10, 100)
	runs, err := l.svcCtx.AgentAudits.ListAgentRuns(l.ctx, repository.AgentRunFilter{Limit: limit})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	traces := make([]*admin.AdminLLMTrace, 0, len(runs))
	for _, run := range runs {
		traces = append(traces, adminTracePB(run))
	}
	recentStates, err := l.svcCtx.Messages.ListRecentConversationStates(l.ctx, limit)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.DashboardResponse{
		Totals: &admin.AdminDashboardTotals{
			Users:         users,
			Conversations: conversations,
			Messages:      messages,
			AiRuns:        aiRuns,
			FailedAiRuns:  failedRuns,
		},
		RecentTraces:        traces,
		RecentConversations: adminConversationsPB(recentStates),
	}, nil
}
