// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateConversationAIHostingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateConversationAIHostingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateConversationAIHostingLogic {
	return &UpdateConversationAIHostingLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateConversationAIHostingLogic) UpdateConversationAIHosting(req *types.UpdateConversationAIHostingReq) (*types.ConversationAIHostingResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.MsgRPC.UpdateConversationAIHosting(l.ctx, &msgpb.UpdateConversationAIHostingRequest{
		OwnerAccountId: userID,
		ConversationId: req.ConversationID,
		Enabled:        req.Enabled,
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
