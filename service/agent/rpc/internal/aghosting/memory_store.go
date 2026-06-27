package aghosting

import (
	"context"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// MemoryStore 是 Store 的内存实现，仅用于单测与 demo/默认 fixture 路径（不接 PG）。
type MemoryStore struct {
	mu       sync.RWMutex
	hosting  map[string]AgentConversationHosting
	triggers map[string]memoryAgentTrigger
	now      func() time.Time
}

type memoryAgentTrigger struct {
	status              string
	responseServerMsgID string
	errorMessage        string
	createdAt           time.Time
	updatedAt           time.Time
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore 构建内存 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		hosting:  make(map[string]AgentConversationHosting),
		triggers: make(map[string]memoryAgentTrigger),
		now:      time.Now,
	}
}

func (s *MemoryStore) UpsertAgentConversationHosting(_ context.Context, hosting AgentConversationHosting) (AgentConversationHosting, error) {
	if err := validateAgentConversationHosting(hosting); err != nil {
		return AgentConversationHosting{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	if existing, ok := s.hosting[hosting.ConversationID]; ok {
		hosting.CreatedAt = existing.CreatedAt
	} else {
		hosting.CreatedAt = now
	}
	hosting.UpdatedAt = now
	s.hosting[hosting.ConversationID] = hosting
	return hosting, nil
}

func (s *MemoryStore) GetAgentConversationHosting(_ context.Context, conversationID string) (AgentConversationHosting, error) {
	if err := validateAgentHostingRequired(conversationID, "conversation_id"); err != nil {
		return AgentConversationHosting{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	hosting, ok := s.hosting[conversationID]
	if !ok {
		return AgentConversationHosting{}, apperror.NotFound("agent conversation hosting not found")
	}
	return hosting, nil
}

func (s *MemoryStore) TryStartAgentTrigger(_ context.Context, input AgentTriggerStartInput) (bool, error) {
	input, err := validateAgentTriggerStartInput(input)
	if err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	existing, ok := s.triggers[input.IdempotencyKey]
	if ok {
		switch existing.status {
		case AgentTriggerStatusSucceeded:
			return false, nil
		case AgentTriggerStatusRunning:
			if !agentTriggerRunningIsStale(existing.updatedAt, now, input.RunningTTL) {
				return false, nil
			}
		case AgentTriggerStatusFailed:
		default:
			return false, nil
		}
	}
	s.triggers[input.IdempotencyKey] = memoryAgentTrigger{
		status:    AgentTriggerStatusRunning,
		createdAt: firstNonZeroAgentHostingTime(existing.createdAt, now),
		updatedAt: now,
	}
	return true, nil
}

func (s *MemoryStore) FinishAgentTrigger(_ context.Context, input AgentTriggerFinishInput) error {
	input, err := validateAgentTriggerFinishInput(input)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	trigger, ok := s.triggers[input.IdempotencyKey]
	if !ok || trigger.status != AgentTriggerStatusRunning {
		return apperror.NotFound("agent trigger idempotency key not found")
	}
	trigger.status = input.Status
	trigger.responseServerMsgID = input.ResponseServerMsgID
	trigger.errorMessage = input.ErrorMessage
	trigger.updatedAt = s.now().UTC()
	s.triggers[input.IdempotencyKey] = trigger
	return nil
}

func agentTriggerRunningIsStale(updatedAt time.Time, now time.Time, ttl time.Duration) bool {
	ttl = normalizeAgentTriggerRunningTTL(ttl)
	if updatedAt.IsZero() {
		return true
	}
	return !updatedAt.UTC().Add(ttl).After(now.UTC())
}

func firstNonZeroAgentHostingTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Time{}
}
