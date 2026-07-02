package orchestrator

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type AgentTriggerReadMarker interface {
	MarkTriggerRead(ctx context.Context, input AgentTriggerReadMark) error
}

type AgentTriggerReadMark struct {
	AccountID      string
	ConversationID string
	TriggerSeq     int64
}

// ConversationReadAdvancer 推进某账号在会话上的已读 seq（脱 internal/repository.SetUserHasReadSeqMax，
// 经 msg-rpc MarkConversationAsRead）。
type ConversationReadAdvancer interface {
	MarkConversationRead(ctx context.Context, accountID, conversationID string, seq int64) error
}

type ConversationReadMarker struct {
	advancer ConversationReadAdvancer
}

func NewConversationReadMarker(advancer ConversationReadAdvancer) ConversationReadMarker {
	return ConversationReadMarker{advancer: advancer}
}

func (m ConversationReadMarker) MarkTriggerRead(ctx context.Context, input AgentTriggerReadMark) error {
	if m.advancer == nil {
		return apperror.Internal("conversation read advancer is not configured")
	}
	accountID, err := normalizeRequired(input.AccountID, "account_id")
	if err != nil {
		return err
	}
	conversationID, err := normalizeRequired(input.ConversationID, "conversation_id")
	if err != nil {
		return err
	}
	if input.TriggerSeq <= 0 {
		return apperror.InvalidArgument("trigger_seq must be greater than 0")
	}
	return m.advancer.MarkConversationRead(ctx, accountID, conversationID, input.TriggerSeq)
}
