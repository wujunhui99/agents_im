package middleware

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// MemorySessionStore is an in-process SessionStore for tests and memory-driver deployments.
type MemorySessionStore struct {
	mu      sync.RWMutex
	entries map[string]memorySessionEntry
	now     func() time.Time
}

type memorySessionEntry struct {
	jti       string
	expiresAt time.Time
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{entries: make(map[string]memorySessionEntry), now: time.Now}
}

func memorySessionKey(userID, device string) string {
	return strings.TrimSpace(userID) + "\x00" + strings.TrimSpace(device)
}

func (s *MemorySessionStore) SetActive(_ context.Context, userID, device, jti string, ttl time.Duration) error {
	userID = strings.TrimSpace(userID)
	jti = strings.TrimSpace(jti)
	if userID == "" || jti == "" {
		return apperror.InvalidArgument("user_id and jti are required")
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[memorySessionKey(userID, device)] = memorySessionEntry{jti: jti, expiresAt: s.now().Add(ttl)}
	return nil
}

func (s *MemorySessionStore) Validate(_ context.Context, userID, device, jti string) error {
	userID = strings.TrimSpace(userID)
	jti = strings.TrimSpace(jti)
	if userID == "" || jti == "" {
		return apperror.Unauthenticated("token session is not active")
	}
	s.mu.RLock()
	entry, ok := s.entries[memorySessionKey(userID, device)]
	s.mu.RUnlock()
	if !ok || entry.jti != jti || !s.now().Before(entry.expiresAt) {
		return apperror.Unauthenticated("token session is not active")
	}
	return nil
}
