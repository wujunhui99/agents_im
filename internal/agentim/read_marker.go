package agentim

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/repository"
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

type MessageRepositoryReadMarker struct {
	repo repository.MessageRepository
}

func NewMessageRepositoryReadMarker(repo repository.MessageRepository) MessageRepositoryReadMarker {
	return MessageRepositoryReadMarker{repo: repo}
}

func (m MessageRepositoryReadMarker) MarkTriggerRead(ctx context.Context, input AgentTriggerReadMark) error {
	if m.repo == nil {
		return apperror.Internal("message repository is not configured")
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
	_, _, err = m.repo.SetUserHasReadSeqMax(ctx, accountID, conversationID, input.TriggerSeq)
	return err
}
