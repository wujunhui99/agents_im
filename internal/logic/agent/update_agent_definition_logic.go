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

type UpdateAgentDefinitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *agentsvc.ServiceContext
}

func NewUpdateAgentDefinitionLogic(ctx context.Context, svcCtx *agentsvc.ServiceContext) *UpdateAgentDefinitionLogic {
	return &UpdateAgentDefinitionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateAgentDefinitionLogic) UpdateAgentDefinition(req *types.UpdateAgentDefinitionReq) (*types.AgentDefinitionResp, error) {
	updatedBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	definition, err := l.svcCtx.AgentDefinitionLogic.UpdateAgentDefinition(l.ctx, business.UpdateAgentDefinitionRequest{
		AgentID:      req.AgentID,
		SystemPrompt: req.SystemPrompt,
		ToolNames:    req.ToolNames,
		UpdatedBy:    updatedBy,
	})
	if err != nil {
		return nil, err
	}
	return agentDefinitionResp(definition), nil
}
