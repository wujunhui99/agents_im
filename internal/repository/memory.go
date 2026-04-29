package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
)

type MemoryRepository struct {
	mu           sync.RWMutex
	nextID       uint64
	byID         map[string]model.User
	identifierID map[string]string
	now          func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byID:         make(map[string]model.User),
		identifierID: make(map[string]string),
		now:          time.Now,
	}
}

func (r *MemoryRepository) Create(_ context.Context, user model.User) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.identifierID[user.Identifier]; exists {
		return model.User{}, apperror.AlreadyExists("identifier already exists")
	}

	r.nextID++
	if user.UserID == "" {
		user.UserID = fmt.Sprintf("usr_%06d", r.nextID)
	}
	now := r.now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	r.byID[user.UserID] = user.Clone()
	r.identifierID[user.Identifier] = user.UserID
	return user.Clone(), nil
}

func (r *MemoryRepository) GetByIdentifier(_ context.Context, identifier string) (model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userID, exists := r.identifierID[identifier]
	if !exists {
		return model.User{}, apperror.NotFound("user not found")
	}

	return r.byID[userID].Clone(), nil
}

func (r *MemoryRepository) ExistsByIdentifier(_ context.Context, identifier string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.identifierID[identifier]
	return exists, nil
}

func (r *MemoryRepository) GetByID(_ context.Context, userID string) (model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.byID[userID]
	if !exists {
		return model.User{}, apperror.NotFound("user not found")
	}

	return user.Clone(), nil
}

func (r *MemoryRepository) UpdateProfile(_ context.Context, userID string, patch ProfilePatch) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.byID[userID]
	if !exists {
		return model.User{}, apperror.NotFound("user not found")
	}

	if patch.DisplayName != nil {
		user.DisplayName = *patch.DisplayName
	}
	if patch.Name != nil {
		user.Name = *patch.Name
	}
	if patch.Gender != nil {
		user.Gender = *patch.Gender
	}
	if patch.Age != nil {
		user.Age = *patch.Age
	}
	if patch.Region != nil {
		user.Region = *patch.Region
	}
	user.UpdatedAt = r.now().UTC()

	r.byID[user.UserID] = user.Clone()
	return user.Clone(), nil
}
