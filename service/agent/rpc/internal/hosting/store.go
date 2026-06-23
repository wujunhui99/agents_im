// Package hosting backs trigger.Judge step 3: "is this conversation AI-hosted,
// and by which agent" (D15). The data owner is the agent domain
// (conversation_ai_hosting); the message main chain never performs this lookup.
// It reads through the agent-owned convhosting.Store (goctl model backed; AG-6 ①
// 数据层脱 internal/repository, D13).
package hosting

import (
	"context"
	"fmt"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
)

// Store implements trigger.HostingStore over the conversation_ai_hosting table.
type Store struct {
	store convhosting.Store
}

// NewStore builds the hosting store. A nil store is a fail-first wiring error.
func NewStore(store convhosting.Store) (*Store, error) {
	if store == nil {
		return nil, fmt.Errorf("hosting store requires a conversation AI hosting store")
	}
	return &Store{store: store}, nil
}

// HostingAgent returns the hosting agent account id when the conversation is
// AI-hosted and enabled. A missing row is "not hosted" (not an error).
func (s *Store) HostingAgent(ctx context.Context, conversationID string) (string, bool, error) {
	setting, err := s.store.GetEnabledConversationAIHosting(ctx, conversationID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	if !setting.Enabled {
		return "", false, nil
	}
	return setting.OwnerAccountID, true, nil
}
