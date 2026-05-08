package agent

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *agentsvc.ServiceContext
}

func NewDeleteAgentLogic(ctx context.Context, svcCtx *agentsvc.ServiceContext) *DeleteAgentLogic {
	return &DeleteAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAgentLogic) DeleteAgent(req *types.AgentPathReq) (*types.AgentResp, error) {
	agent, err := l.svcCtx.AgentLogic.ArchiveAgent(l.ctx, business.AgentPathRequest{AgentID: req.AgentID})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}
