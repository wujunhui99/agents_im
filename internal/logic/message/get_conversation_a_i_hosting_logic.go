package message

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationAIHostingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewGetConversationAIHostingLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *GetConversationAIHostingLogic {
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
	result, err := l.svcCtx.AIHostingLogic.GetConversationAIHosting(l.ctx, business.GetConversationAIHostingRequest{
		OwnerAccountID: userID,
		ConversationID: req.ConversationID,
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
