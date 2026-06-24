package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateAgentStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateAgentStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateAgentStatusLogic {
	return &UpdateAgentStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UpdateAgentStatusLogic) UpdateAgentStatus(req *types.UpdateAgentStatusReq) (*types.AgentResp, error) {
	resp, err := l.svcCtx.AgentRPC.UpdateAgentStatus(l.ctx, &agentpb.UpdateAgentStatusRequest{
		AgentId: req.AgentID,
		Status:  req.Status,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentRespFromPB(resp.GetAgent()), nil
}
