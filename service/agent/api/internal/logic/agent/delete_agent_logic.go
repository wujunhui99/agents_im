package agent

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/types"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAgentLogic {
	return &DeleteAgentLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// DeleteAgent 软删 = 归档（status=archived），经 agent-rpc UpdateAgentStatus（行为对齐旧 ArchiveAgent）。
func (l *DeleteAgentLogic) DeleteAgent(req *types.AgentPathReq) (*types.AgentResp, error) {
	resp, err := l.svcCtx.AgentRPC.UpdateAgentStatus(l.ctx, &agentpb.UpdateAgentStatusRequest{
		AgentId: req.AgentID,
		Status:  model.AgentStatusArchived,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentRespFromPB(resp.GetAgent()), nil
}
