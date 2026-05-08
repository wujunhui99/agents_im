package message

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type SendMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *messagesvc.ServiceContext
}

func NewSendMessageLogic(ctx context.Context, svcCtx *messagesvc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendMessageLogic) SendMessage(req *types.SendMessageReq) (*types.SendMessageResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if senderID := strings.TrimSpace(req.SenderID); senderID != "" && senderID != userID {
		return nil, apperror.InvalidArgument("sender_id must match authenticated user")
	}
	if strings.TrimSpace(req.MessageOrigin) != "" ||
		strings.TrimSpace(req.AgentAccountID) != "" ||
		strings.TrimSpace(req.TriggerServerMsgID) != "" ||
		strings.TrimSpace(req.AgentRunID) != "" ||
		req.AllowRecursiveTrigger {
		return nil, apperror.InvalidArgument("message origin and agent metadata are controlled by Message Service")
	}

	result, err := l.svcCtx.MessageLogic.SendMessage(l.ctx, business.SendMessageRequest{
		SenderID:    userID,
		ReceiverID:  req.ReceiverID,
		GroupID:     req.GroupID,
		ChatType:    req.ChatType,
		ClientMsgID: req.ClientMsgID,
		ContentType: req.ContentType,
		Content:     req.Content,
	})
	if err != nil {
		return nil, err
	}
	return &types.SendMessageResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.SendMessageData{
			Message:      toMessage(result.Message),
			Deduplicated: result.Deduplicated,
		},
	}, nil
}
