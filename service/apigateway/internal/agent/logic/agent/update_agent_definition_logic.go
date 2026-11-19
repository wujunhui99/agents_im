package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/agent/svc"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/agent/types"
	agentpb "github.com/wujunhui99/agents_im_protocol/agent/v1"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAgentDefinitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentDefinitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentDefinitionLogic {
	return &UpdateAgentDefinitionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateAgentDefinitionLogic) UpdateAgentDefinition(req *types.UpdateAgentDefinitionReq) (*types.AgentDefinitionResp, error) {
	updatedBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.AgentRPC.UpdateAgentDefinition(l.ctx, &agentpb.UpdateAgentDefinitionRequest{
		AgentId:      req.AgentID,
		SystemPrompt: req.SystemPrompt,
		ToolNames:    req.ToolNames,
		UpdatedBy:    updatedBy,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentDefinitionRespFromPB(resp.GetDefinition()), nil
}
