package model

import (
	"strings"
	"time"
)

type AccountType string

const (
	AccountTypeNormal AccountType = "normal"
	AccountTypeAgent  AccountType = "agent"
	AccountTypeAdmin  AccountType = "admin"
)

type User struct {
	UserID      string
	Identifier  string
	DisplayName string
	Name        string
	Gender      string
	Age         int32
	Region      string
	AccountType AccountType
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (u User) Clone() User {
	return u
}

func NormalizeAccountType(value string) (AccountType, bool) {
	normalized := AccountType(strings.ToLower(strings.TrimSpace(value)))
	if normalized == "" {
		return AccountTypeNormal, true
	}
	if normalized.IsValid() {
		return normalized, true
	}
	return "", false
}

func (t AccountType) IsValid() bool {
	switch t {
	case AccountTypeNormal, AccountTypeAgent, AccountTypeAdmin:
		return true
	default:
		return false
	}
}
