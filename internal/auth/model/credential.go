package model

import "time"

const (
	PasswordHashVersionBcrypt       = "bcrypt-v1"
	PasswordHashVersionLegacySHA256 = "sha256-iter-v1"
)

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

type ActiveSession struct {
	UserID    string
	SessionID string
	IssuedAt  time.Time
	ExpiresAt time.Time
	UpdatedAt time.Time
}

func (s ActiveSession) Clone() ActiveSession {
	return s
}
