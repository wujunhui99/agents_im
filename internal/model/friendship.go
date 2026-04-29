package model

import "time"

const (
	FriendshipStatusNone    = "none"
	FriendshipStatusActive  = "active"
	FriendshipStatusDeleted = "deleted"
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
