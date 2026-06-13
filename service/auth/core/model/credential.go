package model

import "time"

const (
	PasswordHashVersionBcrypt       = "bcrypt-v1"
	PasswordHashVersionLegacySHA256 = "sha256-iter-v1"
)

type Credential struct {
	Identifier      string
	UserID          string
	Email           string
	EmailVerifiedAt time.Time
	PasswordHash    string
	Salt            string
	HashVersion     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
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

const (
	EmailVerificationPurposeRegister = "register"
)

type EmailVerificationToken struct {
	ID              string
	Purpose         string
	Email           string
	CodeHash        string
	CodeHashVersion string
	ExpiresAt       time.Time
	ConsumedAt      time.Time
	AttemptCount    int
	LastSentAt      time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (t EmailVerificationToken) Clone() EmailVerificationToken {
	return t
}
