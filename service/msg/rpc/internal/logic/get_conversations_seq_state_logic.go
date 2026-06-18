package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetConversationsSeqStateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetConversationsSeqStateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetConversationsSeqStateLogic {
	return &GetConversationsSeqStateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// GetConversationsSeqState 返回会话 max/has_read/unread/末条（重连同步主接口；旧名 GetConversationSeqs）。
func (l *GetConversationsSeqStateLogic) GetConversationsSeqState(in *msg.GetConversationsSeqStateRequest) (*msg.GetConversationsSeqStateResponse, error) {
	userID, err := normalizeMessageRequiredID(in.GetUserId(), "user_id")
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	conversationIDs := make([]string, 0, len(in.GetConversationIds()))
	for _, conversationID := range in.GetConversationIds() {
		normalized, err := normalizeConversationID(conversationID)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		if err := ensureConversationReadAccess(l.ctx, l.svcCtx.Groups, userID, normalized); err != nil {
			return nil, rpcerror.ToStatus(err)
		}
		conversationIDs = append(conversationIDs, normalized)
	}

	states, err := loadConversationSeqStates(l.ctx, l.svcCtx, userID, conversationIDs)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if len(conversationIDs) == 0 {
		states, err = filterReadableConversationStates(l.ctx, l.svcCtx.Groups, userID, states)
		if err != nil {
			return nil, rpcerror.ToStatus(err)
		}
	}

	out := make([]*msg.ConversationSeqState, 0, len(states))
	for _, state := range states {
		out = append(out, seqStateToPB(state))
	}
	return &msg.GetConversationsSeqStateResponse{States: out}, nil
}
