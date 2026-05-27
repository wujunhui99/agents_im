package message

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/service/message/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/message/api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type MarkConversationAsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewMarkConversationAsReadLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *MarkConversationAsReadLogic {
	return &MarkConversationAsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkConversationAsReadLogic) MarkConversationAsRead(req *types.MarkConversationAsReadReq) (*types.MarkConversationAsReadResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MessageLogic.MarkConversationAsRead(l.ctx, business.MarkConversationAsReadRequest{
		UserID:         userID,
		ConversationID: req.ConversationID,
		HasReadSeq:     req.HasReadSeq,
	})
	if err != nil {
		return nil, err
	}
	return &types.MarkConversationAsReadResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.MarkConversationAsReadData{
			ConversationID: result.ConversationID,
			HasReadSeq:     result.HasReadSeq,
			MaxSeq:         result.MaxSeq,
			UnreadCount:    result.UnreadCount,
			Updated:        result.Updated,
		},
	}, nil
}
