package model

import "time"

const (
	FriendshipStatusNone     = "none"
	FriendshipStatusPending  = "pending"
	FriendshipStatusAccepted = "accepted"
	FriendshipStatusRejected = "rejected"
	FriendshipStatusDeleted  = "deleted"

	// FriendshipStatusActive is a V0 compatibility alias for accepted friendships.
	FriendshipStatusActive = FriendshipStatusAccepted
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
