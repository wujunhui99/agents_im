package presence

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	mu          sync.Mutex
	connections map[string]map[string]ConnectionMetadata
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		connections: make(map[string]map[string]ConnectionMetadata),
	}
}

func (s *MemoryStore) RegisterConnection(_ context.Context, metadata ConnectionMetadata, ttl time.Duration) error {
	now := time.Now().UTC()
	metadata, err := normalizeMetadata(metadata, now, ttl)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.connections[metadata.UserID]; !ok {
		s.connections[metadata.UserID] = make(map[string]ConnectionMetadata)
	}
	s.connections[metadata.UserID][metadata.ConnectionID] = metadata
	return nil
}

func (s *MemoryStore) Heartbeat(_ context.Context, userID string, connectionID string, ttl time.Duration) error {
	if err := validateConnectionRef(userID, connectionID, ttl); err != nil {
		return err
	}
	userID = strings.TrimSpace(userID)
	connectionID = strings.TrimSpace(connectionID)

	s.mu.Lock()
	defer s.mu.Unlock()

	userConnections := s.connections[userID]
	metadata, ok := userConnections[connectionID]
	if !ok || metadata.ExpiresAt.Before(time.Now().UTC()) {
		return ErrConnectionNotFound
	}

	now := time.Now().UTC()
	metadata.LastHeartbeatAt = now
	metadata.ExpiresAt = now.Add(ttl)
	userConnections[connectionID] = metadata
	return nil
}

func (s *MemoryStore) UnregisterConnection(_ context.Context, userID string, connectionID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(connectionID) == "" {
		return ErrInvalidConnection
	}
	userID = strings.TrimSpace(userID)
	connectionID = strings.TrimSpace(connectionID)

	s.mu.Lock()
	defer s.mu.Unlock()

	userConnections := s.connections[userID]
	delete(userConnections, connectionID)
	if len(userConnections) == 0 {
		delete(s.connections, userID)
	}
	return nil
}

func (s *MemoryStore) ListUserConnections(_ context.Context, userID string) ([]ConnectionMetadata, error) {
	if !validateUserID(userID) {
		return nil, ErrInvalidConnection
	}
	userID = strings.TrimSpace(userID)

	s.mu.Lock()
	defer s.mu.Unlock()

	userConnections := s.connections[userID]
	if len(userConnections) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	connections := make([]ConnectionMetadata, 0, len(userConnections))
	for connectionID, metadata := range userConnections {
		if !metadata.ExpiresAt.After(now) {
			delete(userConnections, connectionID)
			continue
		}
		connections = append(connections, metadata)
	}
	if len(userConnections) == 0 {
		delete(s.connections, userID)
	}

	sort.Slice(connections, func(i int, j int) bool {
		return connections[i].ConnectionID < connections[j].ConnectionID
	})
	return connections, nil
}

func (s *MemoryStore) IsUserOnline(ctx context.Context, userID string) (bool, error) {
	connections, err := s.ListUserConnections(ctx, userID)
	if err != nil {
		return false, err
	}
	return len(connections) > 0, nil
}
