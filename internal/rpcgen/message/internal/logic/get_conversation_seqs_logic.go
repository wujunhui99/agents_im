package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/svc"
	"github.com/wujunhui99/agents_im/proto/messagepb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationSeqsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationSeqsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationSeqsLogic {
	return &GetConversationSeqsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetConversationSeqsLogic) GetConversationSeqs(in *messagepb.GetConversationSeqsRequest) (*messagepb.GetConversationSeqsResponse, error) {
	// todo: add your logic here and delete this line

	return &messagepb.GetConversationSeqsResponse{}, nil
}
