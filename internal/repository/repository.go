package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/model"
)

type ProfilePatch struct {
	DisplayName *string
	Name        *string
	Gender      *string
	BirthDate   *string
	Region      *string
}

type AccountProfilePatch = ProfilePatch

type AccountRepository interface {
	Create(ctx context.Context, account model.User) (model.User, error)
	GetByIdentifier(ctx context.Context, identifier string) (model.User, error)
	ExistsByIdentifier(ctx context.Context, identifier string) (bool, error)
	GetByID(ctx context.Context, accountID string) (model.User, error)
	ListByAccountType(ctx context.Context, accountType model.AccountType) ([]model.User, error)
	RenameIdentifier(ctx context.Context, fromIdentifier string, toIdentifier string) (model.User, error)
	UpdateProfile(ctx context.Context, accountID string, patch AccountProfilePatch) (model.User, error)
	UpdateAvatar(ctx context.Context, accountID string, avatarMediaID string, avatarURL string) (model.User, error)
}

// UserRepository is the V0 transport compatibility name. It points at account
// profile storage; callers should prefer AccountRepository for new code.
type UserRepository = AccountRepository

type FriendshipRepository interface {
	EnsureAcceptedFriendship(ctx context.Context, userID string, friendID string) error
	AddFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
	AcceptFriendRequest(ctx context.Context, userID string, requesterID string) (model.Friendship, bool, error)
	RejectFriendRequest(ctx context.Context, userID string, requesterID string) (model.Friendship, bool, error)
	DeleteFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
	ListFriends(ctx context.Context, userID string) ([]model.Friendship, error)
	ListIncomingFriendRequests(ctx context.Context, userID string) ([]model.Friendship, error)
	ListOutgoingFriendRequests(ctx context.Context, userID string) ([]model.Friendship, error)
	GetFriendship(ctx context.Context, userID string, friendID string) (model.Friendship, error)
}

type Repository interface {
	AccountRepository
	FriendshipRepository
}
