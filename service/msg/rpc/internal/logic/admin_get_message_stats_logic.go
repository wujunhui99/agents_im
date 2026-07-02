package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminGetMessageStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAdminGetMessageStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetMessageStatsLogic {
	return &AdminGetMessageStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AdminGetMessageStats 返回全域消息/会话总量（admin dashboard 只读，#618）。
func (l *AdminGetMessageStatsLogic) AdminGetMessageStats(in *msg.AdminGetMessageStatsRequest) (*msg.AdminGetMessageStatsResponse, error) {
	messageCount, err := l.svcCtx.Messages.CountMessages(l.ctx)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	conversationCount, err := l.svcCtx.Threads.CountConversations(l.ctx)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &msg.AdminGetMessageStatsResponse{
		MessageCount:      messageCount,
		ConversationCount: conversationCount,
	}, nil
}
