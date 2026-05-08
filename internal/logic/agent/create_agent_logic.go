package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *agentsvc.ServiceContext
}

func NewCreateAgentLogic(ctx context.Context, svcCtx *agentsvc.ServiceContext) *CreateAgentLogic {
	return &CreateAgentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateAgentLogic) CreateAgent(req *types.CreateAgentReq) (*types.AgentResp, error) {
	createdBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	agent, err := l.svcCtx.AgentLogic.CreateAgent(l.ctx, business.CreateAgentRequest{
		IMUserID:    req.IMUserID,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return nil, err
	}
	return agentResp(agent), nil
}
