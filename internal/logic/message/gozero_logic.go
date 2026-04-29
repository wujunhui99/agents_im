package message

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationSeqsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetConversationSeqsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationSeqsLogic {
	return &GetConversationSeqsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetConversationSeqsLogic) GetConversationSeqs(req *types.ConversationSeqsReq) (*types.ConversationSeqsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MessageLogic.GetConversationSeqs(l.ctx, business.GetConversationSeqsRequest{
		UserID:          userID,
		ConversationIDs: splitCommaQuery(req.ConversationIDs),
	})
	if err != nil {
		return nil, err
	}

	states := make([]types.ConversationSeqState, 0, len(result.States))
	for _, state := range result.States {
		states = append(states, toConversationSeqState(state))
	}
	return &types.ConversationSeqsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ConversationSeqsData{States: states},
	}, nil
}

type MarkConversationAsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkConversationAsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkConversationAsReadLogic {
	return &MarkConversationAsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkConversationAsReadLogic) MarkConversationAsRead(req *types.MarkConversationAsReadReq) (*types.MarkConversationAsReadResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.MessageLogic.MarkConversationAsRead(l.ctx, business.MarkConversationAsReadRequest{
		UserID:         userID,
		ConversationID: req.ConversationID,
		HasReadSeq:     req.HasReadSeq,
	})
	if err != nil {
		return nil, err
	}
	return &types.MarkConversationAsReadResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.MarkConversationAsReadData{
			ConversationID: result.ConversationID,
			HasReadSeq:     result.HasReadSeq,
			MaxSeq:         result.MaxSeq,
			UnreadCount:    result.UnreadCount,
			Updated:        result.Updated,
		},
	}, nil
}

type PullMessagesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPullMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PullMessagesLogic {
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

func (l *SendMessageLogic) SendMessage(req *types.SendMessageReq) (*types.SendMessageResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if senderID := strings.TrimSpace(req.SenderID); senderID != "" && senderID != userID {
		return nil, apperror.InvalidArgument("sender_id must match authenticated user")
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

func splitCommaQuery(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	rawParts := strings.Split(value, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func toConversationSeqState(state business.ConversationSeqState) types.ConversationSeqState {
	var lastMessage *types.Message
	if state.LastMessage != nil {
		msg := toMessage(*state.LastMessage)
		lastMessage = &msg
	}
	return types.ConversationSeqState{
		ConversationID: state.ConversationID,
		MaxSeq:         state.MaxSeq,
		HasReadSeq:     state.HasReadSeq,
		UnreadCount:    state.UnreadCount,
		MaxSeqTime:     state.MaxSeqTime,
		LastMessage:    lastMessage,
	}
}

func toMessage(message business.Message) types.Message {
	return types.Message{
		ServerMsgID:    message.ServerMsgID,
		ClientMsgID:    message.ClientMsgID,
		ConversationID: message.ConversationID,
		Seq:            message.Seq,
		SenderID:       message.SenderID,
		ReceiverID:     message.ReceiverID,
		GroupID:        message.GroupID,
		ChatType:       message.ChatType,
		ContentType:    message.ContentType,
		Content:        message.Content,
		SendTime:       message.SendTime,
		CreatedAt:      message.CreatedAt,
	}
}
