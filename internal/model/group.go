package model

import "time"

const (
	MemberStateActive = "active"
	MemberStateLeft   = "left"
)

type Group struct {
	GroupID       string
	Name          string
	Description   string
	CreatorUserID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (g Group) Clone() Group {
	return g
}

type GroupMember struct {
	GroupID  string
	UserID   string
	State    string
	JoinedAt time.Time
	LeftAt   time.Time
}

func (m GroupMember) Clone() GroupMember {
	return m
}
