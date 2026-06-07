package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationMessagesLogic {
	return &GetConversationMessagesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// GetConversationMessages 按 seq 区间分页拉取某会话消息（管理员视角，内容脱敏）。
func (l *GetConversationMessagesLogic) GetConversationMessages(in *admin.ConversationMessagesRequest) (*admin.ConversationMessagesResponse, error) {
	if l.svcCtx.Messages == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("admin message repository is not configured"))
	}
	conversationID, err := validateRequiredAdminID(in.GetConversationId(), "conversation_id", 256)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	limit := normalizeAdminLimit(int(in.GetLimit()), 50, 500)
	order := strings.ToLower(strings.TrimSpace(in.GetOrder()))
	if order == "" {
		order = repository.MessageStorageOrderAsc
	}
	messages, isEnd, nextSeq, err := l.svcCtx.Messages.GetMessages(l.ctx, conversationID, in.GetFromSeq(), in.GetToSeq(), limit, order)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &admin.ConversationMessagesResponse{
		ConversationId: conversationID,
		Messages:       adminMessagesPB(messages),
		IsEnd:          isEnd,
		NextSeq:        nextSeq,
	}, nil
}
