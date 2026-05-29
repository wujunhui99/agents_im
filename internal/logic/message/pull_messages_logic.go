package message

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type PullMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewPullMessagesLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *PullMessagesLogic {
	return &PullMessagesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PullMessagesLogic) PullMessages(req *types.PullMessagesReq) (*types.PullMessagesResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MessageLogic.PullMessages(l.ctx, business.PullMessagesRequest{
		UserID:         userID,
		ConversationID: req.ConversationID,
		FromSeq:        req.FromSeq,
		ToSeq:          req.ToSeq,
		Limit:          int(req.Limit),
		Order:          req.Order,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]types.Message, 0, len(result.Messages))
	for _, msg := range result.Messages {
		messages = append(messages, toMessage(msg))
	}
	return &types.PullMessagesResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.PullMessagesData{
			Messages: messages,
			IsEnd:    result.IsEnd,
			NextSeq:  result.NextSeq,
		},
	}, nil
}
