// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendMessageLogic) SendMessage(req *types.SendMessageReq) (resp *types.SendMessageResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if senderID := strings.TrimSpace(req.SenderID); senderID != "" && senderID != userID {
		return nil, apperror.InvalidArgument("sender_id must match authenticated user")
	}
	// message_origin / agent 元数据由 Message Service 控制，HTTP 入口不允许指定。
	if strings.TrimSpace(req.MessageOrigin) != "" ||
		strings.TrimSpace(req.AgentAccountID) != "" ||
		strings.TrimSpace(req.TriggerServerMsgID) != "" ||
		strings.TrimSpace(req.AgentRunID) != "" ||
		req.AllowRecursiveTrigger {
		return nil, apperror.InvalidArgument("message origin and agent metadata are controlled by Message Service")
	}

	result, err := l.svcCtx.MsgRPC.SendMessage(l.ctx, &msgpb.SendMessageRequest{
		SenderId:    userID,
		ReceiverId:  req.ReceiverID,
		GroupId:     req.GroupID,
		ChatType:    req.ChatType,
		ClientMsgId: req.ClientMsgID,
		ContentType: req.ContentType,
		Content:     req.Content,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.SendMessageResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.SendMessageData{
			Message:      pbToMessage(result.GetMessage()),
			Deduplicated: result.GetDeduplicated(),
		},
	}, nil
}
