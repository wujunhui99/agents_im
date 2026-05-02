package model

import (
	"strings"
	"time"
)

type AccountType string

const (
	AccountTypeUser  AccountType = "user"
	AccountTypeAgent AccountType = "agent"
	AccountTypeAdmin AccountType = "admin"

	// AccountTypeNormal is a temporary V0 compatibility alias for older code and
	// persisted rows that used "normal" before the domain moved to Account.
	AccountTypeNormal AccountType = AccountTypeUser
)

type Account = User

type User struct {
	UserID        string
	Identifier    string
	DisplayName   string
	Name          string
	Gender        string
	Age           int32
	Region        string
	AccountType   AccountType
	AvatarMediaID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (u User) Clone() User {
	return u
}

func NormalizeAccountType(value string) (AccountType, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch AccountType(normalized) {
	case "", AccountTypeUser:
		return AccountTypeUser, true
	case "normal":
		return AccountTypeUser, true
	case AccountTypeAgent:
		return AccountTypeAgent, true
	case AccountTypeAdmin:
		return AccountTypeAdmin, true
	default:
		return "", false
	}
}

func (t AccountType) IsValid() bool {
	switch t {
	case AccountTypeUser, AccountTypeAgent, AccountTypeAdmin:
		return true
	default:
		return false
	}
}
