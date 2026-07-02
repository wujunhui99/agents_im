package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/rpc/admin"
	"github.com/wujunhui99/agents_im/service/admin/rpc/internal/svc"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"

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
// 会话消息原始读经属主 msg-rpc.AdminGetConversationMessages（#618，脱 internal/repository）；
// 区间/limit/order 归一化由 msg-rpc 侧统一处理。
func (l *GetConversationMessagesLogic) GetConversationMessages(in *admin.ConversationMessagesRequest) (*admin.ConversationMessagesResponse, error) {
	conversationID, err := validateRequiredAdminID(in.GetConversationId(), "conversation_id", 256)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	resp, err := l.svcCtx.MsgRPC.AdminGetConversationMessages(l.ctx, &msgpb.AdminGetConversationMessagesRequest{
		ConversationId: conversationID,
		FromSeq:        in.GetFromSeq(),
		ToSeq:          in.GetToSeq(),
		Limit:          in.GetLimit(),
		Order:          in.GetOrder(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	return &admin.ConversationMessagesResponse{
		ConversationId: conversationID,
		Messages:       adminMessagesPB(resp.GetMessages()),
		IsEnd:          resp.GetIsEnd(),
		NextSeq:        resp.GetNextSeq(),
	}, nil
}
