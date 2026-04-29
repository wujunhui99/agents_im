package model

import "time"

type Credential struct {
	Identifier   string
	UserID       string
	PasswordHash string
	Salt         string
	HashVersion  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c Credential) Clone() Credential {
	return c
}
