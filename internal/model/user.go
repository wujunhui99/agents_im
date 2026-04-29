package model

import "time"

type User struct {
	UserID      string
	Identifier  string
	DisplayName string
	Name        string
	Gender      string
	Age         int32
	Region      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u User) Clone() User {
	return u
}
