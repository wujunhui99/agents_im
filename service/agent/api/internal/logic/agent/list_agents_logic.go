package agent

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListAgentsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListAgentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListAgentsLogic {
	return &ListAgentsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListAgentsLogic) ListAgents(req *types.ListAgentsReq) (*types.ListAgentsResp, error) {
	result, err := l.svcCtx.AgentLogic.ListAgents(l.ctx, business.ListAgentsRequest{
		Status:    req.Status,
		CreatedBy: req.CreatedBy,
		Limit:     int(req.Limit),
		Offset:    int(req.Offset),
	})
	if err != nil {
		return nil, err
	}
	agents := make([]types.Agent, 0, len(result.Agents))
	for _, item := range result.Agents {
		agents = append(agents, agentType(item))
	}
	return &types.ListAgentsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ListAgentsData{
			Agents: agents,
		},
	}, nil
}
