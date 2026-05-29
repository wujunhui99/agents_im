package repository

import (
	"context"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type MemoryConversationAIHostingRepository struct {
	mu       sync.RWMutex
	settings map[string]ConversationAIHostingSetting
	now      func() time.Time
}

func NewMemoryConversationAIHostingRepository() *MemoryConversationAIHostingRepository {
	return &MemoryConversationAIHostingRepository{
		settings: make(map[string]ConversationAIHostingSetting),
		now:      time.Now,
	}
}

func (r *MemoryConversationAIHostingRepository) GetConversationAIHostingSetting(_ context.Context, ownerAccountID string, conversationID string) (ConversationAIHostingSetting, error) {
	ownerAccountID, conversationID, err := normalizeConversationAIHostingOwnerAndConversation(ownerAccountID, conversationID)
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	setting, ok := r.settings[conversationAIHostingSettingKey(ownerAccountID, conversationID)]
	if !ok {
		return ConversationAIHostingSetting{}, apperror.NotFound("conversation AI hosting setting not found")
	}
	return setting.Clone(), nil
}

func (r *MemoryConversationAIHostingRepository) GetEnabledConversationAIHosting(_ context.Context, conversationID string) (ConversationAIHostingSetting, error) {
	conversationID, err := normalizeAgentHostingRequired(conversationID, "conversation_id")
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, setting := range r.settings {
		if setting.ConversationID == conversationID && setting.Enabled {
			return setting.Clone(), nil
		}
	}
	return ConversationAIHostingSetting{}, apperror.NotFound("enabled conversation AI hosting setting not found")
}

func (r *MemoryConversationAIHostingRepository) SetConversationAIHostingEnabled(_ context.Context, input ConversationAIHostingUpdate) (ConversationAIHostingSetting, error) {
	input, err := normalizeConversationAIHostingUpdate(input)
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if input.Enabled {
		for _, setting := range r.settings {
			if setting.ConversationID == input.ConversationID && setting.Enabled && setting.OwnerAccountID != input.OwnerAccountID {
				return ConversationAIHostingSetting{}, conversationAIHostingConflictError()
			}
		}
	}

	now := r.now().UTC()
	key := conversationAIHostingSettingKey(input.OwnerAccountID, input.ConversationID)
	setting, ok := r.settings[key]
	if !ok {
		setting = ConversationAIHostingSetting{
			OwnerAccountID: input.OwnerAccountID,
			ConversationID: input.ConversationID,
			CreatedAt:      now,
		}
	}
	setting.Enabled = input.Enabled
	setting.Mode = ConversationAIHostingModeAutoReply
	setting.MaxRecentMessages = normalizeConversationAIHostingRecentLimit(input.MaxRecentMessages)
	setting.SummaryEnabled = input.SummaryEnabled
	setting.UpdatedAt = now

	r.settings[key] = setting.Clone()
	return setting.Clone(), nil
}

func conversationAIHostingSettingKey(ownerAccountID string, conversationID string) string {
	return ownerAccountID + "\x00" + conversationID
}
