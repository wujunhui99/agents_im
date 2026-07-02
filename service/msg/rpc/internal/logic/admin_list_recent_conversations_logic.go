package logic

import (
	"context"
	"errors"
	"strconv"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListRecentConversationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminListRecentConversationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListRecentConversationsLogic {
	return &AdminListRecentConversationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AdminListRecentConversations 按最近活跃倒序返回会话概要（含末条消息，admin dashboard 只读，#618）。
func (l *AdminListRecentConversationsLogic) AdminListRecentConversations(in *msg.AdminListRecentConversationsRequest) (*msg.AdminListRecentConversationsResponse, error) {
	rows, err := l.svcCtx.Threads.ListRecentConversations(l.ctx, int(in.GetLimit()))
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	out := make([]*msg.ConversationSeqState, 0, len(rows))
	for _, row := range rows {
		state := model.ConversationSeqState{
			ConversationID: row.ConversationID,
			MaxSeq:         row.MaxSeq,
		}
		if row.LastMessageID != "" {
			lastMessage, err := l.lookupLastMessage(row.LastMessageID)
			if err != nil {
				return nil, rpcerror.ToStatus(err)
			}
			if lastMessage != nil {
				state.LastMessage = lastMessage
				state.MaxSeqTime = model.MessageSendTime(lastMessage)
			}
		}
		out = append(out, seqStateToPB(state))
	}
	return &msg.AdminListRecentConversationsResponse{States: out}, nil
}

// lookupLastMessage 按 last_message_id（十进制 message_id）取末条消息；已被清理的引用视为无末条。
func (l *AdminListRecentConversationsLogic) lookupLastMessage(lastMessageID string) (*model.Messages, error) {
	messageID, err := strconv.ParseInt(lastMessageID, 10, 64)
	if err != nil {
		return nil, nil
	}
	message, err := l.svcCtx.Messages.FindOne(l.ctx, messageID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return message, nil
}
