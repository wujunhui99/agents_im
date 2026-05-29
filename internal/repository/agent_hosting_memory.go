package repository

import (
	"context"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type MemoryAgentConversationHostingRepository struct {
	mu       sync.RWMutex
	hosting  map[string]AgentConversationHosting
	triggers map[string]memoryAgentTrigger
	now      func() time.Time
}

type memoryAgentTrigger struct {
	input               AgentTriggerStartInput
	status              string
	responseServerMsgID string
	errorMessage        string
	createdAt           time.Time
	updatedAt           time.Time
}

func NewMemoryAgentConversationHostingRepository() *MemoryAgentConversationHostingRepository {
	return &MemoryAgentConversationHostingRepository{
		hosting:  make(map[string]AgentConversationHosting),
		triggers: make(map[string]memoryAgentTrigger),
		now:      time.Now,
	}
}

func (r *MemoryAgentConversationHostingRepository) UpsertAgentConversationHosting(_ context.Context, hosting AgentConversationHosting) (AgentConversationHosting, error) {
	hosting, err := normalizeAgentConversationHosting(hosting)
	if err != nil {
		return AgentConversationHosting{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	if existing, ok := r.hosting[hosting.ConversationID]; ok {
		hosting.CreatedAt = existing.CreatedAt
	} else {
		hosting.CreatedAt = now
	}
	hosting.UpdatedAt = now
	r.hosting[hosting.ConversationID] = hosting.Clone()
	return hosting.Clone(), nil
}

func (r *MemoryAgentConversationHostingRepository) GetAgentConversationHosting(_ context.Context, conversationID string) (AgentConversationHosting, error) {
	conversationID, err := normalizeAgentHostingRequired(conversationID, "conversation_id")
	if err != nil {
		return AgentConversationHosting{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	hosting, ok := r.hosting[conversationID]
	if !ok {
		return AgentConversationHosting{}, apperror.NotFound("agent conversation hosting not found")
	}
	return hosting.Clone(), nil
}

func (r *MemoryAgentConversationHostingRepository) TryStartAgentTrigger(_ context.Context, input AgentTriggerStartInput) (bool, error) {
	input, err := normalizeAgentTriggerStartInput(input)
	if err != nil {
		return false, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	existing, ok := r.triggers[input.IdempotencyKey]
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
	r.triggers[input.IdempotencyKey] = memoryAgentTrigger{
		input:     input,
		status:    AgentTriggerStatusRunning,
		createdAt: firstNonZeroRepositoryTime(existing.createdAt, now),
		updatedAt: now,
	}
	return true, nil
}

func (r *MemoryAgentConversationHostingRepository) FinishAgentTrigger(_ context.Context, input AgentTriggerFinishInput) error {
	input, err := normalizeAgentTriggerFinishInput(input)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	trigger, ok := r.triggers[input.IdempotencyKey]
	if !ok {
		return apperror.NotFound("agent trigger idempotency key not found")
	}
	trigger.status = input.Status
	trigger.responseServerMsgID = input.ResponseServerMsgID
	trigger.errorMessage = input.ErrorMessage
	trigger.updatedAt = r.now().UTC()
	r.triggers[input.IdempotencyKey] = trigger
	return nil
}

func agentTriggerRunningIsStale(updatedAt time.Time, now time.Time, ttl time.Duration) bool {
	ttl = normalizeAgentTriggerRunningTTL(ttl)
	if updatedAt.IsZero() {
		return true
	}
	return !updatedAt.UTC().Add(ttl).After(now.UTC())
}

func firstNonZeroRepositoryTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value.UTC()
		}
	}
	return time.Time{}
}
