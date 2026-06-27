package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/model"
)

type AccountSearchFilter struct {
	Query string
	Limit int
}

type AdminAccountRepository interface {
	GetByID(ctx context.Context, accountID string) (model.User, error)
	SearchAccounts(ctx context.Context, filter AccountSearchFilter) ([]model.User, error)
	CountAccounts(ctx context.Context) (int64, error)
}

type AdminMessageRepository interface {
	GetMessages(ctx context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error)
	GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error)
	CountMessages(ctx context.Context) (int64, error)
	CountConversations(ctx context.Context) (int64, error)
	ListRecentConversationStates(ctx context.Context, limit int) ([]ConversationSeqState, error)
}
