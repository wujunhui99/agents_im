package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetConversationMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminGetConversationMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetConversationMessagesLogic {
	return &AdminGetConversationMessagesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AdminGetConversationMessages 按 seq 区间原始读某会话消息（管理员视角，#618）。
// 与 PullMessages 的关键区别：无用户可见边界裁剪（管理员不是会话成员），直接以
// conversation_threads.max_seq 为上界读全量区间；脱敏由调用方 admin-rpc 负责。
func (l *AdminGetConversationMessagesLogic) AdminGetConversationMessages(in *msg.AdminGetConversationMessagesRequest) (*msg.AdminGetConversationMessagesResponse, error) {
	conversationID, err := normalizeConversationID(in.GetConversationId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	fromSeq, toSeq, limit, order, err := normalizePullRange(in.GetFromSeq(), in.GetToSeq(), int(in.GetLimit()), in.GetOrder())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	maxSeq, err := l.svcCtx.Threads.GetMaxSeq(l.ctx, conversationID)
	if err != nil {
		// 会话不存在（无 thread 行）→ 空结果（管理员对任意会话都可查，不存在即无消息）。
		if errors.Is(err, model.ErrNotFound) {
			return &msg.AdminGetConversationMessagesResponse{IsEnd: true, NextSeq: fromSeq}, nil
		}
		return nil, rpcerror.ToStatus(err)
	}

	rows, isEnd, nextSeq, err := l.svcCtx.Messages.GetMessagesInRange(l.ctx, conversationID, fromSeq, toSeq, maxSeq, limit, order)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	out := make([]*msg.Message, 0, len(rows))
	for _, m := range rows {
		out = append(out, messageToPB(m))
	}
	return &msg.AdminGetConversationMessagesResponse{Messages: out, IsEnd: isEnd, NextSeq: nextSeq}, nil
}
