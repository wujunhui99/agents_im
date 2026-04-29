package repository

import (
	"context"
	"fmt"
	"sort"
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
	friendships  map[string]model.Friendship
	now          func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		byID:         make(map[string]model.User),
		identifierID: make(map[string]string),
		friendships:  make(map[string]model.Friendship),
		now:          time.Now,
	}
}

func (r *MemoryRepository) Create(_ context.Context, user model.User) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.identifierID[user.Identifier]; exists {
		return model.User{}, apperror.AlreadyExists("identifier already exists")
	}
	accountType, ok := model.NormalizeAccountType(string(user.AccountType))
	if !ok {
		return model.User{}, apperror.InvalidArgument("account_type must be normal, agent, or admin")
	}
	user.AccountType = accountType

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

func (r *MemoryRepository) AddFriend(_ context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := friendshipKey(userID, friendID)
	if existing, exists := r.friendships[key]; exists && existing.Status == model.FriendshipStatusActive {
		return existing.Clone(), false, nil
	}

	now := r.now().UTC()
	friendship := model.Friendship{
		UserID:    userID,
		FriendID:  friendID,
		Status:    model.FriendshipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	reverse := model.Friendship{
		UserID:    friendID,
		FriendID:  userID,
		Status:    model.FriendshipStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.friendships[key] = friendship.Clone()
	r.friendships[friendshipKey(friendID, userID)] = reverse.Clone()
	return friendship.Clone(), true, nil
}

func (r *MemoryRepository) DeleteFriend(_ context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := friendshipKey(userID, friendID)
	existing, exists := r.friendships[key]
	if !exists || existing.Status != model.FriendshipStatusActive {
		return model.Friendship{}, false, apperror.NotFound("friendship not found")
	}

	now := r.now().UTC()
	existing.Status = model.FriendshipStatusDeleted
	existing.UpdatedAt = now
	r.friendships[key] = existing.Clone()

	reverseKey := friendshipKey(friendID, userID)
	reverse, reverseExists := r.friendships[reverseKey]
	if reverseExists {
		reverse.Status = model.FriendshipStatusDeleted
		reverse.UpdatedAt = now
		r.friendships[reverseKey] = reverse.Clone()
	}

	return existing.Clone(), true, nil
}

func (r *MemoryRepository) ListFriends(_ context.Context, userID string) ([]model.Friendship, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	friendships := make([]model.Friendship, 0)
	for _, friendship := range r.friendships {
		if friendship.UserID == userID && friendship.Status == model.FriendshipStatusActive {
			friendships = append(friendships, friendship.Clone())
		}
	}

	sort.Slice(friendships, func(i int, j int) bool {
		return friendships[i].FriendID < friendships[j].FriendID
	})
	return friendships, nil
}

func (r *MemoryRepository) GetFriendship(_ context.Context, userID string, friendID string) (model.Friendship, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	friendship, exists := r.friendships[friendshipKey(userID, friendID)]
	if !exists {
		return model.Friendship{}, apperror.NotFound("friendship not found")
	}

	return friendship.Clone(), nil
}

func friendshipKey(userID string, friendID string) string {
	return userID + "\x00" + friendID
}
