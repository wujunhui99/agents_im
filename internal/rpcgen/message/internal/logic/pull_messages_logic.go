package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/svc"
	"github.com/wujunhui99/agents_im/proto/messagepb"

	"github.com/zeromicro/go-zero/core/logx"
)

type PullMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPullMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PullMessagesLogic {
	return &PullMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *PullMessagesLogic) PullMessages(in *messagepb.PullMessagesRequest) (*messagepb.PullMessagesResponse, error) {
	// todo: add your logic here and delete this line

	return &messagepb.PullMessagesResponse{}, nil
}
