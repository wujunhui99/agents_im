package repository

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/model"
)

type MemoryRepository struct {
	mu           sync.RWMutex
	byIdentifier map[string]model.Credential
	byUserID     map[string]string
	sessions     map[string]model.ActiveSession
	now          func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byIdentifier: make(map[string]model.Credential),
		byUserID:     make(map[string]string),
		sessions:     make(map[string]model.ActiveSession),
		now:          time.Now,
	}
}

func (r *MemoryRepository) Create(_ context.Context, credential model.Credential) (model.Credential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byIdentifier[credential.Identifier]; exists {
		return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
	}
	if _, exists := r.byUserID[credential.UserID]; exists {
		return model.Credential{}, apperror.AlreadyExists("auth credential already exists")
	}

	now := r.now().UTC()
	credential.CreatedAt = now
	credential.UpdatedAt = now

	r.byIdentifier[credential.Identifier] = credential.Clone()
	r.byUserID[credential.UserID] = credential.Identifier
	return credential.Clone(), nil
}

func (r *MemoryRepository) SetActiveSession(_ context.Context, session model.ActiveSession) error {
	session.UserID = strings.TrimSpace(session.UserID)
	session.SessionID = strings.TrimSpace(session.SessionID)
	if session.UserID == "" || session.SessionID == "" {
		return apperror.InvalidArgument("active session requires user_id and session_id")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byUserID[session.UserID]; !exists {
		return apperror.NotFound("auth credential not found")
	}
	now := r.now().UTC()
	session.IssuedAt = session.IssuedAt.UTC()
	session.ExpiresAt = session.ExpiresAt.UTC()
	session.UpdatedAt = now
	r.sessions[session.UserID] = session.Clone()
	return nil
}

func (r *MemoryRepository) GetActiveSession(_ context.Context, userID string) (model.ActiveSession, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return model.ActiveSession{}, apperror.InvalidArgument("user_id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.sessions[userID]
	if !exists {
		return model.ActiveSession{}, apperror.NotFound("active session not found")
	}
	return session.Clone(), nil
}

func (r *MemoryRepository) GetByIdentifier(_ context.Context, identifier string) (model.Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	credential, exists := r.byIdentifier[identifier]
	if !exists {
		return model.Credential{}, apperror.NotFound("auth credential not found")
	}

	return credential.Clone(), nil
}
