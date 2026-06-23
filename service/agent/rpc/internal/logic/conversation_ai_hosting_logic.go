// Package logic holds agent-rpc gRPC handlers. AI 托管开关 CRUD 复用 agent 域自有的
// convhosting.ConversationAIHostingLogic 业务规则（同一双人单聊只允许一方开启等），
// 数据 owner = agent 域（#340 从 msg.proto/msg-rpc 迁入；AG-6 ① 数据层脱 internal）。
package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// ---- GetConversationAIHosting ----

type GetConversationAIHostingLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationAIHostingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationAIHostingLogic {
	return &GetConversationAIHostingLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetConversationAIHostingLogic) GetConversationAIHosting(in *agent.GetConversationAIHostingRequest) (*agent.ConversationAIHostingState, error) {
	hostingLogic := l.aiHostingLogic()
	if hostingLogic == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("conversation AI hosting is not configured"))
	}
	result, err := hostingLogic.GetConversationAIHosting(l.ctx, convhosting.GetConversationAIHostingRequest{
		OwnerAccountID: in.GetOwnerAccountId(),
		ConversationID: in.GetConversationId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return aiHostingStateToPB(result), nil
}

func (l *GetConversationAIHostingLogic) aiHostingLogic() *convhosting.ConversationAIHostingLogic {
	if l.svcCtx == nil || l.svcCtx.Hosting == nil {
		return nil
	}
	return l.svcCtx.Hosting.AIHostingLogic
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

func (l *UpdateConversationAIHostingLogic) UpdateConversationAIHosting(in *agent.UpdateConversationAIHostingRequest) (*agent.ConversationAIHostingState, error) {
	if l.svcCtx == nil || l.svcCtx.Hosting == nil || l.svcCtx.Hosting.AIHostingLogic == nil {
		return nil, rpcerror.ToStatus(apperror.Internal("conversation AI hosting is not configured"))
	}
	result, err := l.svcCtx.Hosting.AIHostingLogic.UpdateConversationAIHosting(l.ctx, convhosting.UpdateConversationAIHostingRequest{
		OwnerAccountID: in.GetOwnerAccountId(),
		ConversationID: in.GetConversationId(),
		Enabled:        in.GetEnabled(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return aiHostingStateToPB(result), nil
}

func aiHostingStateToPB(s convhosting.ConversationAIHostingResponse) *agent.ConversationAIHostingState {
	return &agent.ConversationAIHostingState{
		ConversationId:    s.ConversationID,
		ChatType:          s.ChatType,
		Enabled:           s.Enabled,
		Available:         s.Available,
		PeerEnabled:       s.PeerEnabled,
		UnavailableReason: s.UnavailableReason,
		MaxRecentMessages: int64(s.MaxRecentMessages),
		SummaryEnabled:    s.SummaryEnabled,
	}
}
