package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/service/message/rpc/internal/svc"
	messagepb "github.com/wujunhui99/agents_im/service/message/rpc/message"

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
	result, err := l.svcCtx.MessageLogic.PullMessages(l.ctx, business.PullMessagesRequest{
		UserID:         in.GetUserId(),
		ConversationID: in.GetConversationId(),
		FromSeq:        in.GetFromSeq(),
		ToSeq:          in.GetToSeq(),
		Limit:          int(in.GetLimit()),
		Order:          in.GetOrder(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	messages := make([]*messagepb.Message, 0, len(result.Messages))
	for _, message := range result.Messages {
		messages = append(messages, toMessage(message))
	}
	return &messagepb.PullMessagesResponse{
		Messages: messages,
		IsEnd:    result.IsEnd,
		NextSeq:  result.NextSeq,
	}, nil
}
