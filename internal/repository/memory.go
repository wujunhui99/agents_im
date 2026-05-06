package repository

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/idgen"
	"github.com/wujunhui99/agents_im/internal/model"
)

type MemoryRepository struct {
	mu           sync.RWMutex
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
		return model.User{}, apperror.InvalidArgument("account_type must be user, agent, or admin")
	}
	user.AccountType = accountType

	if user.UserID == "" {
		accountID, err := idgen.NewString()
		if err != nil {
			return model.User{}, err
		}
		user.UserID = accountID
	}
	if user.AccountID == "" {
		user.AccountID = user.UserID
	}
	user.UserID = user.AccountID
	now := r.now().UTC()
	account := model.Account{
		AccountID:   user.AccountID,
		Identifier:  user.Identifier,
		AccountType: user.AccountType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	profile := model.Profile{
		AccountID:     user.AccountID,
		DisplayName:   user.DisplayName,
		Name:          user.Name,
		Gender:        user.Gender,
		BirthDate:     user.BirthDate,
		Region:        user.Region,
		AvatarMediaID: user.AvatarMediaID,
		AvatarURL:     user.AvatarURL,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	user = model.NewAccountProfile(account, profile)

	r.byID[user.AccountID] = user.Clone()
	r.identifierID[user.Identifier] = user.AccountID
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
	if patch.BirthDate != nil {
		user.BirthDate = *patch.BirthDate
	}
	if patch.Region != nil {
		user.Region = *patch.Region
	}
	user.ProfileUpdatedAt = r.now().UTC()
	user.UpdatedAt = user.ProfileUpdatedAt

	r.byID[user.AccountID] = user.Clone()
	return user.Clone(), nil
}

func (r *MemoryRepository) UpdateAvatar(_ context.Context, userID string, avatarMediaID string, avatarURL string) (model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.byID[userID]
	if !exists {
		return model.User{}, apperror.NotFound("user not found")
	}

	user.AvatarMediaID = strings.TrimSpace(avatarMediaID)
	user.AvatarURL = strings.TrimSpace(avatarURL)
	user.ProfileUpdatedAt = r.now().UTC()
	user.UpdatedAt = user.ProfileUpdatedAt
	r.byID[user.AccountID] = user.Clone()
	return user.Clone(), nil
}

func (r *MemoryRepository) AddFriend(_ context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := friendshipKey(userID, friendID)
	if existing, exists := r.friendships[key]; exists {
		if existing.Status == model.FriendshipStatusAccepted || existing.Status == model.FriendshipStatusPending {
			return existing.Clone(), false, nil
		}
	}

	now := r.now().UTC()
	reverseKey := friendshipKey(friendID, userID)
	reverse, reverseExists := r.friendships[reverseKey]
	if reverseExists && reverse.Status == model.FriendshipStatusPending {
		friendship := model.Friendship{
			UserID:    userID,
			FriendID:  friendID,
			Status:    model.FriendshipStatusAccepted,
			CreatedAt: now,
			UpdatedAt: now,
		}
		reverse.Status = model.FriendshipStatusAccepted
		reverse.UpdatedAt = now
		r.friendships[key] = friendship.Clone()
		r.friendships[reverseKey] = reverse.Clone()
		return friendship.Clone(), true, nil
	}

	friendship := model.Friendship{
		UserID:    userID,
		FriendID:  friendID,
		Status:    model.FriendshipStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.friendships[key] = friendship.Clone()
	return friendship.Clone(), true, nil
}

func (r *MemoryRepository) AcceptFriendRequest(_ context.Context, userID string, requesterID string) (model.Friendship, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	incomingKey := friendshipKey(requesterID, userID)
	incoming, exists := r.friendships[incomingKey]
	if !exists || incoming.Status != model.FriendshipStatusPending {
		return model.Friendship{}, false, apperror.NotFound("friend request not found")
	}

	now := r.now().UTC()
	incoming.Status = model.FriendshipStatusAccepted
	incoming.UpdatedAt = now
	if incoming.CreatedAt.IsZero() {
		incoming.CreatedAt = now
	}

	currentKey := friendshipKey(userID, requesterID)
	current := model.Friendship{
		UserID:    userID,
		FriendID:  requesterID,
		Status:    model.FriendshipStatusAccepted,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing, ok := r.friendships[currentKey]; ok {
		current.CreatedAt = existing.CreatedAt
		if current.CreatedAt.IsZero() {
			current.CreatedAt = now
		}
	}

	r.friendships[incomingKey] = incoming.Clone()
	r.friendships[currentKey] = current.Clone()
	return current.Clone(), true, nil
}

func (r *MemoryRepository) RejectFriendRequest(_ context.Context, userID string, requesterID string) (model.Friendship, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	incomingKey := friendshipKey(requesterID, userID)
	incoming, exists := r.friendships[incomingKey]
	if !exists || incoming.Status != model.FriendshipStatusPending {
		return model.Friendship{}, false, apperror.NotFound("friend request not found")
	}

	now := r.now().UTC()
	incoming.Status = model.FriendshipStatusRejected
	incoming.UpdatedAt = now
	if incoming.CreatedAt.IsZero() {
		incoming.CreatedAt = now
	}

	currentKey := friendshipKey(userID, requesterID)
	current := model.Friendship{
		UserID:    userID,
		FriendID:  requesterID,
		Status:    model.FriendshipStatusRejected,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing, ok := r.friendships[currentKey]; ok {
		current.CreatedAt = existing.CreatedAt
		if current.CreatedAt.IsZero() {
			current.CreatedAt = now
		}
	}

	r.friendships[incomingKey] = incoming.Clone()
	r.friendships[currentKey] = current.Clone()
	return current.Clone(), true, nil
}

func (r *MemoryRepository) DeleteFriend(_ context.Context, userID string, friendID string) (model.Friendship, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := friendshipKey(userID, friendID)
	existing, exists := r.friendships[key]
	if !exists || existing.Status != model.FriendshipStatusAccepted {
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
		if friendship.UserID == userID && friendship.Status == model.FriendshipStatusAccepted {
			friendships = append(friendships, friendship.Clone())
		}
	}

	sort.Slice(friendships, func(i int, j int) bool {
		return friendships[i].FriendID < friendships[j].FriendID
	})
	return friendships, nil
}

func (r *MemoryRepository) ListIncomingFriendRequests(_ context.Context, userID string) ([]model.Friendship, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	friendships := make([]model.Friendship, 0)
	for _, friendship := range r.friendships {
		if friendship.FriendID == userID && friendship.Status == model.FriendshipStatusPending {
			friendships = append(friendships, friendship.Clone())
		}
	}
	sort.Slice(friendships, func(i int, j int) bool {
		return friendships[i].UserID < friendships[j].UserID
	})
	return friendships, nil
}

func (r *MemoryRepository) ListOutgoingFriendRequests(_ context.Context, userID string) ([]model.Friendship, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	friendships := make([]model.Friendship, 0)
	for _, friendship := range r.friendships {
		if friendship.UserID == userID && friendship.Status == model.FriendshipStatusPending {
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
	if exists {
		return friendship.Clone(), nil
	}

	// Pending requests are stored once as requester -> recipient. Return a
	// synthetic reverse pending view so the recipient can still query status
	// against the requester without turning it into an accepted friendship.
	reverse, reverseExists := r.friendships[friendshipKey(friendID, userID)]
	if reverseExists && reverse.Status == model.FriendshipStatusPending {
		return model.Friendship{
			UserID:    userID,
			FriendID:  friendID,
			Status:    model.FriendshipStatusPending,
			CreatedAt: reverse.CreatedAt,
			UpdatedAt: reverse.UpdatedAt,
		}, nil
	}

	return model.Friendship{}, apperror.NotFound("friendship not found")
}

func friendshipKey(userID string, friendID string) string {
	return userID + "\x00" + friendID
}
