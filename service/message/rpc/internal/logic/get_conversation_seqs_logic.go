package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/service/message/rpc/internal/svc"
	messagepb "github.com/wujunhui99/agents_im/service/message/rpc/message"

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
	result, err := l.svcCtx.MessageLogic.GetConversationSeqs(l.ctx, business.GetConversationSeqsRequest{
		UserID:          in.GetUserId(),
		ConversationIDs: in.GetConversationIds(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	states := make([]*messagepb.ConversationSeqState, 0, len(result.States))
	for _, state := range result.States {
		states = append(states, toConversationSeqState(state))
	}
	return &messagepb.GetConversationSeqsResponse{States: states}, nil
}
