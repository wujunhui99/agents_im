package agent

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *agentsvc.ServiceContext
}

func NewGetAgentLogic(ctx context.Context, svcCtx *agentsvc.ServiceContext) *GetAgentLogic {
	return &GetAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAgentLogic) GetAgent(req *types.AgentPathReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.GetAgent(l.ctx, business.AgentPathRequest{AgentID: req.AgentID})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}
