package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentLogic {
	return &UpdateAgentLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateAgentLogic) UpdateAgent(req *types.UpdateAgentReq) (*types.AgentResp, error) {
	resp, err := l.svcCtx.AgentRPC.UpdateAgent(l.ctx, &agentpb.UpdateAgentRequest{
		AgentId:     req.AgentID,
		Name:        optionalString(req.Name),
		Description: optionalString(req.Description),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentRespFromPB(resp.GetAgent()), nil
}
