package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListAgentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentsLogic {
	return &ListAgentsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListAgentsLogic) ListAgents(req *types.ListAgentsReq) (*types.ListAgentsResp, error) {
	resp, err := l.svcCtx.AgentRPC.ListAgents(l.ctx, &agentpb.ListAgentsRequest{
		Status:    req.Status,
		CreatedBy: req.CreatedBy,
		Limit:     req.Limit,
		Offset:    req.Offset,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	agents := make([]types.Agent, 0, len(resp.GetAgents()))
	for _, item := range resp.GetAgents() {
		agents = append(agents, agentTypeFromPB(item))
	}
	return &types.ListAgentsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ListAgentsData{Agents: agents},
	}, nil
}
