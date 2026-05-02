package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/model"
)

type ProfilePatch struct {
	DisplayName *string
	Name        *string
	Gender      *string
	Age         *int32
	Region      *string
}

type AccountProfilePatch = ProfilePatch

type AccountRepository interface {
	Create(ctx context.Context, account model.User) (model.User, error)
	GetByIdentifier(ctx context.Context, identifier string) (model.User, error)
	ExistsByIdentifier(ctx context.Context, identifier string) (bool, error)
	GetByID(ctx context.Context, accountID string) (model.User, error)
	UpdateProfile(ctx context.Context, accountID string, patch AccountProfilePatch) (model.User, error)
	UpdateAvatar(ctx context.Context, accountID string, avatarMediaID string) (model.User, error)
}

// UserRepository is the V0 transport compatibility name. It points at account
// profile storage; callers should prefer AccountRepository for new code.
type UserRepository = AccountRepository

type FriendshipRepository interface {
	AddFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
	DeleteFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
	ListFriends(ctx context.Context, userID string) ([]model.Friendship, error)
	GetFriendship(ctx context.Context, userID string, friendID string) (model.Friendship, error)
}

type Repository interface {
	AccountRepository
	FriendshipRepository
}
