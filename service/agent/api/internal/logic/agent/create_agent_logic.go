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

type CreateAgentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateAgentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateAgentLogic {
	return &CreateAgentLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateAgentLogic) CreateAgent(req *types.CreateAgentReq) (*types.AgentResp, error) {
	createdBy, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	resp, err := l.svcCtx.AgentRPC.CreateAgent(l.ctx, &agentpb.CreateAgentRequest{
		ImUserId:    req.IMUserID,
		Name:        req.Name,
		Description: req.Description,
		Status:      req.Status,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return agentRespFromPB(resp.GetAgent()), nil
}
