package agent

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentLogic {
	return &UpdateAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAgentLogic) UpdateAgent(req *types.UpdateAgentReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.UpdateAgent(l.ctx, business.UpdateAgentRequest{
		AgentID:     req.AgentID,
		Name:        optionalAgentString(req.Name),
		Description: optionalAgentString(req.Description),
	})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}
