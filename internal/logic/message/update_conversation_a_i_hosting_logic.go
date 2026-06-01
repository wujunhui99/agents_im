package message

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/common/share/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateConversationAIHostingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewUpdateConversationAIHostingLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *UpdateConversationAIHostingLogic {
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
	result, err := l.svcCtx.AIHostingLogic.UpdateConversationAIHosting(l.ctx, business.UpdateConversationAIHostingRequest{
		OwnerAccountID: userID,
		ConversationID: req.ConversationID,
		Enabled:        req.Enabled,
	})
	if err != nil {
		return nil, err
	}
	return &types.ConversationAIHostingResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    toConversationAIHostingData(result),
	}, nil
}
