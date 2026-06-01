package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/svc"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/proto/messagepb"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkConversationAsReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkConversationAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkConversationAsReadLogic {
	return &MarkConversationAsReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkConversationAsReadLogic) MarkConversationAsRead(in *messagepb.MarkConversationAsReadRequest) (*messagepb.MarkConversationAsReadResponse, error) {
	result, err := l.svcCtx.MessageLogic.MarkConversationAsRead(l.ctx, business.MarkConversationAsReadRequest{
		UserID:         in.GetUserId(),
		ConversationID: in.GetConversationId(),
		HasReadSeq:     in.GetHasReadSeq(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &messagepb.MarkConversationAsReadResponse{
		ConversationId: result.ConversationID,
		HasReadSeq:     result.HasReadSeq,
		MaxSeq:         result.MaxSeq,
		UnreadCount:    result.UnreadCount,
		Updated:        result.Updated,
	}, nil
}
