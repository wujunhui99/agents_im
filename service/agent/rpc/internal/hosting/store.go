// Package hosting backs trigger.Judge step 3: "is this conversation AI-hosted,
// and by which agent" (D15). The data owner is the agent domain
// (conversation_ai_hosting); the message main chain never performs this lookup.
// Today it reads through the keystone internal/repository data layer; it moves
// to a service/agent/rpc/internal/model goctl model when AG-6 (D13) lands.
package hosting

import (
	"context"
	"fmt"

	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// Store implements trigger.HostingStore over the conversation_ai_hosting table.
type Store struct {
	repo repository.ConversationAIHostingRepository
}

// NewStore builds the hosting store. A nil repository is a fail-first wiring error.
func NewStore(repo repository.ConversationAIHostingRepository) (*Store, error) {
	if repo == nil {
		return nil, fmt.Errorf("hosting store requires a conversation AI hosting repository")
	}
	return &Store{repo: repo}, nil
}

// HostingAgent returns the hosting agent account id when the conversation is
// AI-hosted and enabled. A missing row is "not hosted" (not an error).
func (s *Store) HostingAgent(ctx context.Context, conversationID string) (string, bool, error) {
	setting, err := s.repo.GetEnabledConversationAIHosting(ctx, conversationID)
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
