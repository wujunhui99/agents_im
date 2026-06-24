package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetAgentLogic {
	return &GetAgentLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetAgentLogic) GetAgent(req *types.AgentPathReq) (*types.AgentResp, error) {
	resp, err := l.svcCtx.AgentRPC.GetAgent(l.ctx, &agentpb.GetAgentRequest{AgentId: req.AgentID})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentRespFromPB(resp.GetAgent()), nil
}
