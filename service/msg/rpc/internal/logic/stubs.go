package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Phase 0 未实现的 RPC（07-message-rpc-redesign §8 分阶段落地）。统一返回 Unimplemented，
// 接口面已在 proto 固定，待后续 Phase 填实现。

func unimplemented(name string) error {
	return status.Errorf(codes.Unimplemented, "%s is not implemented yet (msg-rpc Phase 0)", name)
}

// ---- AppendStreamMessage（P1: Agent 流式回写）----
type AppendStreamMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAppendStreamMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AppendStreamMessageLogic {
	return &AppendStreamMessageLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AppendStreamMessageLogic) AppendStreamMessage(in *msg.AppendStreamMessageRequest) (*msg.AppendStreamMessageResponse, error) {
	return nil, unimplemented("AppendStreamMessage")
}

// ---- GetLastMessageByConvs（侧边栏批量末条）----
type GetLastMessageByConvsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetLastMessageByConvsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLastMessageByConvsLogic {
	return &GetLastMessageByConvsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetLastMessageByConvsLogic) GetLastMessageByConvs(in *msg.GetLastMessageByConvsRequest) (*msg.GetLastMessageByConvsResponse, error) {
	return nil, unimplemented("GetLastMessageByConvs")
}

// ---- GetMaxSeqs ----
type GetMaxSeqsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetMaxSeqsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetMaxSeqsLogic {
	return &GetMaxSeqsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetMaxSeqsLogic) GetMaxSeqs(in *msg.GetMaxSeqsRequest) (*msg.GetMaxSeqsResponse, error) {
	return nil, unimplemented("GetMaxSeqs")
}

// ---- GetHasReadSeqs ----
type GetHasReadSeqsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHasReadSeqsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHasReadSeqsLogic {
	return &GetHasReadSeqsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetHasReadSeqsLogic) GetHasReadSeqs(in *msg.GetHasReadSeqsRequest) (*msg.GetHasReadSeqsResponse, error) {
	return nil, unimplemented("GetHasReadSeqs")
}

// ---- RevokeMessage（微信式撤回，需 messages.revoked 字段 + 撤回事件）----
type RevokeMessageLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRevokeMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RevokeMessageLogic {
	return &RevokeMessageLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *RevokeMessageLogic) RevokeMessage(in *msg.RevokeMessageRequest) (*msg.RevokeMessageResponse, error) {
	return nil, unimplemented("RevokeMessage")
}

// ---- DeleteMessages ----
type DeleteMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMessagesLogic {
	return &DeleteMessagesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DeleteMessagesLogic) DeleteMessages(in *msg.DeleteMessagesRequest) (*msg.DeleteMessagesResponse, error) {
	return nil, unimplemented("DeleteMessages")
}

// ---- ClearConversationMessages ----
type ClearConversationMessagesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClearConversationMessagesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClearConversationMessagesLogic {
	return &ClearConversationMessagesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ClearConversationMessagesLogic) ClearConversationMessages(in *msg.ClearConversationMessagesRequest) (*msg.ClearConversationMessagesResponse, error) {
	return nil, unimplemented("ClearConversationMessages")
}

// ---- GetServerTime ----
type GetServerTimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetServerTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetServerTimeLogic {
	return &GetServerTimeLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetServerTimeLogic) GetServerTime(in *msg.GetServerTimeRequest) (*msg.GetServerTimeResponse, error) {
	return nil, unimplemented("GetServerTime")
}
