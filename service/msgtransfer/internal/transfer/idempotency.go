package transfer

import (
	"context"
	"sync"
)

type NoopIdempotencyStore struct{}

func (NoopIdempotencyStore) HasProcessed(context.Context, string) (bool, error) {
	return false, nil
}

func (NoopIdempotencyStore) MarkProcessed(context.Context, string) error {
	return nil
}

type MemoryIdempotencyStore struct {
	mu        sync.RWMutex
	processed map[string]struct{}
}

func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		processed: make(map[string]struct{}),
	}
}

func (s *MemoryIdempotencyStore) HasProcessed(_ context.Context, key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.processed[key]
	return ok, nil
}

func (s *MemoryIdempotencyStore) MarkProcessed(_ context.Context, key string) error {
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[key] = struct{}{}
	return nil
}
