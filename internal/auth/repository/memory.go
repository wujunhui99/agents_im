package repository

import (
	"context"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/model"
)

type MemoryRepository struct {
	mu           sync.RWMutex
	byIdentifier map[string]model.Credential
	byUserID     map[string]string
	now          func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byIdentifier: make(map[string]model.Credential),
		byUserID:     make(map[string]string),
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

func (r *MemoryRepository) GetByIdentifier(_ context.Context, identifier string) (model.Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	credential, exists := r.byIdentifier[identifier]
	if !exists {
		return model.Credential{}, apperror.NotFound("auth credential not found")
	}

	return credential.Clone(), nil
}
