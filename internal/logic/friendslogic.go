package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type UserLookup interface {
	GetUserByID(ctx context.Context, req GetUserByIDRequest) (UserProfile, error)
}

type FriendsLogic struct {
	repo  repository.FriendshipRepository
	users UserLookup
}

func NewFriendsLogic(repo repository.FriendshipRepository, users UserLookup) *FriendsLogic {
	return &FriendsLogic{repo: repo, users: users}
}

type FriendshipView struct {
	UserID    string       `json:"user_id"`
	FriendID  string       `json:"friend_id"`
	Status    string       `json:"status"`
	IsFriend  bool         `json:"is_friend"`
	Friend    *UserProfile `json:"friend,omitempty"`
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
}

type AddFriendRequest struct {
	UserID   string `json:"user_id"`
	FriendID string `json:"friend_id"`
}

type AddFriendResponse struct {
	Friendship FriendshipView `json:"friendship"`
	Created    bool           `json:"created"`
}

type DeleteFriendRequest struct {
	UserID   string `json:"user_id"`
	FriendID string `json:"friend_id"`
}

type DeleteFriendResponse struct {
	Friendship FriendshipView `json:"friendship"`
	Deleted    bool           `json:"deleted"`
}

type ListFriendsRequest struct {
	UserID string `json:"user_id"`
}

type ListFriendsResponse struct {
	Friends []FriendshipView `json:"friends"`
}

type GetFriendshipRequest struct {
	UserID   string `json:"user_id"`
	FriendID string `json:"friend_id"`
}

type GetFriendshipResponse struct {
	Friendship FriendshipView `json:"friendship"`
}

func (l *FriendsLogic) AddFriend(ctx context.Context, req AddFriendRequest) (AddFriendResponse, error) {
	userID, friendID, err := normalizeFriendshipPair(req.UserID, req.FriendID)
	if err != nil {
		return AddFriendResponse{}, err
	}

	if err := l.ensureUsersExist(ctx, userID, friendID); err != nil {
		return AddFriendResponse{}, err
	}

	friendship, created, err := l.repo.AddFriend(ctx, userID, friendID)
	if err != nil {
		return AddFriendResponse{}, err
	}

	return AddFriendResponse{
		Friendship: toFriendshipView(friendship),
		Created:    created,
	}, nil
}

func (l *FriendsLogic) DeleteFriend(ctx context.Context, req DeleteFriendRequest) (DeleteFriendResponse, error) {
	userID, friendID, err := normalizeFriendshipPair(req.UserID, req.FriendID)
	if err != nil {
		return DeleteFriendResponse{}, err
	}

	if err := l.ensureUsersExist(ctx, userID, friendID); err != nil {
		return DeleteFriendResponse{}, err
	}

	friendship, deleted, err := l.repo.DeleteFriend(ctx, userID, friendID)
	if err != nil {
		return DeleteFriendResponse{}, err
	}

	return DeleteFriendResponse{
		Friendship: toFriendshipView(friendship),
		Deleted:    deleted,
	}, nil
}

func (l *FriendsLogic) ListFriends(ctx context.Context, req ListFriendsRequest) (ListFriendsResponse, error) {
	userID := normalizeUserID(req.UserID)
	if userID == "" {
		return ListFriendsResponse{}, apperror.InvalidArgument("user_id is required")
	}

	if err := l.ensureUserExists(ctx, userID); err != nil {
		return ListFriendsResponse{}, err
	}

	friendships, err := l.repo.ListFriends(ctx, userID)
	if err != nil {
		return ListFriendsResponse{}, err
	}

	friends := make([]FriendshipView, 0, len(friendships))
	for _, friendship := range friendships {
		view := toFriendshipView(friendship)
		if profile, profileErr := l.lookupFriendProfile(ctx, friendship.FriendID); profileErr == nil {
			view.Friend = &profile
		} else {
			return ListFriendsResponse{}, profileErr
		}
		friends = append(friends, view)
	}

	return ListFriendsResponse{Friends: friends}, nil
}

func (l *FriendsLogic) GetFriendship(ctx context.Context, req GetFriendshipRequest) (GetFriendshipResponse, error) {
	userID, friendID, err := normalizeFriendshipPair(req.UserID, req.FriendID)
	if err != nil {
		return GetFriendshipResponse{}, err
	}

	if err := l.ensureUsersExist(ctx, userID, friendID); err != nil {
		return GetFriendshipResponse{}, err
	}

	friendship, err := l.repo.GetFriendship(ctx, userID, friendID)
	if err != nil {
		appErr := apperror.From(err)
		if appErr.Code != apperror.CodeNotFound {
			return GetFriendshipResponse{}, err
		}
		friendship = model.Friendship{
			UserID:   userID,
			FriendID: friendID,
			Status:   model.FriendshipStatusNone,
		}
	}

	return GetFriendshipResponse{Friendship: toFriendshipView(friendship)}, nil
}

func (l *FriendsLogic) ensureUsersExist(ctx context.Context, userID string, friendID string) error {
	if err := l.ensureUserExists(ctx, userID); err != nil {
		return err
	}
	return l.ensureUserExists(ctx, friendID)
}

func (l *FriendsLogic) lookupFriendProfile(ctx context.Context, userID string) (UserProfile, error) {
	if l.users == nil {
		return UserProfile{}, apperror.Internal("user lookup is not configured")
	}
	return l.users.GetUserByID(ctx, GetUserByIDRequest{UserID: userID})
}

func (l *FriendsLogic) ensureUserExists(ctx context.Context, userID string) error {
	_, err := l.lookupFriendProfile(ctx, userID)
	return err
}

func normalizeFriendshipPair(userID string, friendID string) (string, string, error) {
	userID = normalizeUserID(userID)
	friendID = normalizeUserID(friendID)
	if userID == "" {
		return "", "", apperror.InvalidArgument("user_id is required")
	}
	if friendID == "" {
		return "", "", apperror.InvalidArgument("friend_id is required")
	}
	if userID == friendID {
		return "", "", apperror.InvalidArgument("user_id and friend_id must be different")
	}
	return userID, friendID, nil
}

func normalizeUserID(userID string) string {
	return strings.TrimSpace(userID)
}

func toFriendshipView(friendship model.Friendship) FriendshipView {
	isFriend := friendship.Status == model.FriendshipStatusActive
	return FriendshipView{
		UserID:    friendship.UserID,
		FriendID:  friendship.FriendID,
		Status:    friendship.Status,
		IsFriend:  isFriend,
		CreatedAt: formatTime(friendship.CreatedAt),
		UpdatedAt: formatTime(friendship.UpdatedAt),
	}
}
