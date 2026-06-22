// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationAIHostingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConversationAIHostingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationAIHostingLogic {
	return &GetConversationAIHostingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConversationAIHostingLogic) GetConversationAIHosting(req *types.ConversationAIHostingReq) (*types.ConversationAIHostingResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.AgentRPC.GetConversationAIHosting(l.ctx, &agentpb.GetConversationAIHostingRequest{
		OwnerAccountId: userID,
		ConversationId: req.ConversationID,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.ConversationAIHostingResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    pbToAIHostingData(result),
	}, nil
}
