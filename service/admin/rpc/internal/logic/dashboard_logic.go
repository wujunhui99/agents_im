package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

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
	usersResp, err := l.svcCtx.UserRPC.CountAccounts(l.ctx, &userpb.CountAccountsRequest{})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	users := usersResp.GetCount()
	// 消息/会话总量经属主 msg-rpc（#618，脱 internal/repository AdminMessageRepository）。
	statsResp, err := l.svcCtx.MsgRPC.AdminGetMessageStats(l.ctx, &msgpb.AdminGetMessageStatsRequest{})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	conversations := statsResp.GetConversationCount()
	messages := statsResp.GetMessageCount()
	aiRunsResp, err := l.svcCtx.AgentRPC.CountAgentRuns(l.ctx, &agent.CountAgentRunsRequest{})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	aiRuns := aiRunsResp.GetCount()
	failedRunsResp, err := l.svcCtx.AgentRPC.CountAgentRuns(l.ctx, &agent.CountAgentRunsRequest{Status: string(agentaudit.StatusFailed)})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	failedRuns := failedRunsResp.GetCount()
	limit := normalizeAdminLimit(int(in.GetLimit()), 10, 100)
	runsResp, err := l.svcCtx.AgentRPC.ListAgentRuns(l.ctx, &agent.ListAgentRunsRequest{Limit: int64(limit)})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	traces := make([]*admin.AdminLLMTrace, 0, len(runsResp.GetRuns()))
	for _, run := range runsResp.GetRuns() {
		traces = append(traces, adminTracePB(agentRunFromPB(run)))
	}
	recentResp, err := l.svcCtx.MsgRPC.AdminListRecentConversations(l.ctx, &msgpb.AdminListRecentConversationsRequest{Limit: int32(limit)})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
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
		RecentConversations: adminConversationsPB(recentResp.GetStates()),
	}, nil
}
