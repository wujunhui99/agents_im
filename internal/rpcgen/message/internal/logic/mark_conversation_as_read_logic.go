package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/svc"
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
	// todo: add your logic here and delete this line

	return &messagepb.MarkConversationAsReadResponse{}, nil
}
