package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

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

// PullMessages 按用户视角拉历史（Redis 优先留待 Phase 2；Phase 0 走 PG，行为对齐旧实现）。
func (l *PullMessagesLogic) PullMessages(in *msg.PullMessagesRequest) (*msg.PullMessagesResponse, error) {
	userID, err := normalizeMessageRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	conversationID, err := normalizeConversationID(in.GetConversationId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := ensureConversationReadAccess(l.ctx, l.svcCtx.Groups, userID, conversationID); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	fromSeq, toSeq, limit, order, err := normalizePullRange(in.GetFromSeq(), in.GetToSeq(), int(in.GetLimit()), in.GetOrder())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	states, err := loadConversationSeqStates(l.ctx, l.svcCtx, userID, []string{conversationID})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if len(states) == 1 {
		if toSeq == 0 || toSeq > states[0].MaxSeq {
			toSeq = states[0].MaxSeq
		}
	}

	messages, isEnd, nextSeq, err := loadMessagesForUser(l.ctx, l.svcCtx, userID, conversationID, fromSeq, toSeq, limit, order)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	out := make([]*msg.Message, 0, len(messages))
	for _, m := range messages {
		out = append(out, messageToPB(m))
	}
	return &msg.PullMessagesResponse{Messages: out, IsEnd: isEnd, NextSeq: nextSeq}, nil
}
