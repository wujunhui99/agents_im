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

type UserRepository interface {
	Create(ctx context.Context, user model.User) (model.User, error)
	GetByIdentifier(ctx context.Context, identifier string) (model.User, error)
	ExistsByIdentifier(ctx context.Context, identifier string) (bool, error)
	GetByID(ctx context.Context, userID string) (model.User, error)
	UpdateProfile(ctx context.Context, userID string, patch ProfilePatch) (model.User, error)
	UpdateAvatar(ctx context.Context, userID string, avatarMediaID string) (model.User, error)
}

type FriendshipRepository interface {
	AddFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
	DeleteFriend(ctx context.Context, userID string, friendID string) (model.Friendship, bool, error)
	ListFriends(ctx context.Context, userID string) ([]model.Friendship, error)
	GetFriendship(ctx context.Context, userID string, friendID string) (model.Friendship, error)
}

type Repository interface {
	UserRepository
	FriendshipRepository
}
