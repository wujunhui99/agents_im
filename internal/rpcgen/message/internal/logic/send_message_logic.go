package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/message/internal/svc"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/proto/messagepb"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SendMessageLogic) SendMessage(in *messagepb.SendMessageRequest) (*messagepb.SendMessageResponse, error) {
	result, err := l.svcCtx.MessageLogic.SendMessage(l.ctx, business.SendMessageRequest{
		SenderID:              in.GetSenderId(),
		ReceiverID:            in.GetReceiverId(),
		GroupID:               in.GetGroupId(),
		ChatType:              in.GetChatType(),
		ClientMsgID:           in.GetClientMsgId(),
		ContentType:           in.GetContentType(),
		Content:               in.GetContent(),
		MessageOrigin:         in.GetMessageOrigin(),
		AgentAccountID:        in.GetAgentAccountId(),
		TriggerServerMsgID:    in.GetTriggerServerMsgId(),
		AgentRunID:            in.GetAgentRunId(),
		AllowRecursiveTrigger: in.GetAllowRecursiveTrigger(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &messagepb.SendMessageResponse{
		Message:      toMessage(result.Message),
		Deduplicated: result.Deduplicated,
	}, nil
}
