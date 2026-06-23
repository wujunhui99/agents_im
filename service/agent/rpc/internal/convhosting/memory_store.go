package convhosting

import (
	"context"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// MemoryStore 是 Store 的内存实现，仅用于单测与 demo/默认 fixture 路径（不接 PG）。
type MemoryStore struct {
	mu       sync.RWMutex
	settings map[string]Setting
	now      func() time.Time
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore 构建内存 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		settings: make(map[string]Setting),
		now:      time.Now,
	}
}

func (s *MemoryStore) GetConversationAIHostingSetting(_ context.Context, ownerAccountID string, conversationID string) (Setting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	setting, ok := s.settings[settingKey(ownerAccountID, conversationID)]
	if !ok {
		return Setting{}, apperror.NotFound("conversation AI hosting setting not found")
	}
	return setting, nil
}

func (s *MemoryStore) GetEnabledConversationAIHosting(_ context.Context, conversationID string) (Setting, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, setting := range s.settings {
		if setting.ConversationID == conversationID && setting.Enabled {
			return setting, nil
		}
	}
	return Setting{}, apperror.NotFound("enabled conversation AI hosting setting not found")
}

func (s *MemoryStore) SetConversationAIHostingEnabled(_ context.Context, input Update) (Setting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if input.Enabled {
		for _, setting := range s.settings {
			if setting.ConversationID == input.ConversationID && setting.Enabled && setting.OwnerAccountID != input.OwnerAccountID {
				return Setting{}, conflictError()
			}
		}
	}

	now := s.now().UTC()
	key := settingKey(input.OwnerAccountID, input.ConversationID)
	setting, ok := s.settings[key]
	if !ok {
		setting = Setting{
			OwnerAccountID: input.OwnerAccountID,
			ConversationID: input.ConversationID,
			CreatedAt:      now,
		}
	}
	setting.Enabled = input.Enabled
	setting.Mode = modeAutoReply
	setting.MaxRecentMessages = clampRecentMessages(input.MaxRecentMessages)
	setting.SummaryEnabled = input.SummaryEnabled
	setting.UpdatedAt = now

	s.settings[key] = setting
	return setting, nil
}

func settingKey(ownerAccountID string, conversationID string) string {
	return ownerAccountID + "\x00" + conversationID
}
