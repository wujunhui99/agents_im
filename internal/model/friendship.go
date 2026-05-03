package model

import "time"

const (
	FriendshipStatusNone     = "none"
	FriendshipStatusPending  = "pending"
	FriendshipStatusAccepted = "accepted"
	FriendshipStatusRejected = "rejected"
	FriendshipStatusActive   = "active"
	FriendshipStatusDeleted  = "deleted"
)

type Friendship struct {
	UserID    string
	FriendID  string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (f Friendship) Clone() Friendship {
	return f
}

func IsAcceptedFriendshipStatus(status string) bool {
	return status == FriendshipStatusAccepted || status == FriendshipStatusActive
}
