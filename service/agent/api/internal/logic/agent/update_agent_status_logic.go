package agent

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAgentStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentStatusLogic {
	return &UpdateAgentStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAgentStatusLogic) UpdateAgentStatus(req *types.UpdateAgentStatusReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.UpdateAgentStatus(l.ctx, business.UpdateAgentStatusRequest{
		AgentID: req.AgentID,
		Status:  req.Status,
	})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}
