// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAgentDefinitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *agentsvc.ServiceContext
}

func NewGetAgentDefinitionLogic(ctx context.Context, svcCtx *agentsvc.ServiceContext) *GetAgentDefinitionLogic {
	return &GetAgentDefinitionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAgentDefinitionLogic) GetAgentDefinition(req *types.AgentPathReq) (*types.AgentDefinitionResp, error) {
	requestedBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	definition, err := l.svcCtx.AgentDefinitionLogic.GetAgentDefinition(l.ctx, business.AgentDefinitionRequest{
		AgentID:     req.AgentID,
		RequestedBy: requestedBy,
	})
	if err != nil {
		return nil, err
	}
	return agentDefinitionResp(definition), nil
}
