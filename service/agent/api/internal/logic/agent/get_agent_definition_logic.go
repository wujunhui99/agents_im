package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAgentDefinitionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAgentDefinitionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentDefinitionLogic {
	return &GetAgentDefinitionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetAgentDefinitionLogic) GetAgentDefinition(req *types.AgentPathReq) (*types.AgentDefinitionResp, error) {
	requestedBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.AgentRPC.GetAgentDefinition(l.ctx, &agentpb.GetAgentDefinitionRequest{
		AgentId:     req.AgentID,
		RequestedBy: requestedBy,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentDefinitionRespFromPB(resp.GetDefinition()), nil
}
