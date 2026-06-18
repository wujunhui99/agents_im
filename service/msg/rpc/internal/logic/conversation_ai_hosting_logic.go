package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

// AI 托管开关 RPC（keystone 例外：随 message-api 退役落到 msg-rpc，复用 internal
// ConversationAIHostingLogic 业务规则；待 agent 域 rpc / 03 §9 B1 后迁出）。

// ---- GetConversationAIHosting ----

type GetConversationAIHostingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationAIHostingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationAIHostingLogic {
	return &GetConversationAIHostingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetConversationAIHostingLogic) GetConversationAIHosting(in *msg.GetConversationAIHostingRequest) (*msg.ConversationAIHostingState, error) {
	if l.svcCtx.AIHosting == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("conversation AI hosting is not configured"))
	}
	result, err := l.svcCtx.AIHosting.GetConversationAIHosting(l.ctx, business.GetConversationAIHostingRequest{
		OwnerAccountID: in.GetOwnerAccountId(),
		ConversationID: in.GetConversationId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return aiHostingStateToPB(result), nil
}

// ---- UpdateConversationAIHosting ----

type UpdateConversationAIHostingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateConversationAIHostingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateConversationAIHostingLogic {
	return &UpdateConversationAIHostingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *UpdateConversationAIHostingLogic) UpdateConversationAIHosting(in *msg.UpdateConversationAIHostingRequest) (*msg.ConversationAIHostingState, error) {
	if l.svcCtx.AIHosting == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("conversation AI hosting is not configured"))
	}
	result, err := l.svcCtx.AIHosting.UpdateConversationAIHosting(l.ctx, business.UpdateConversationAIHostingRequest{
		OwnerAccountID: in.GetOwnerAccountId(),
		ConversationID: in.GetConversationId(),
		Enabled:        in.GetEnabled(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return aiHostingStateToPB(result), nil
}
