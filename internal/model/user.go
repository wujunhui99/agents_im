package model

import (
	"time"
)

type AccountType int32

const (
	AccountTypeAdmin AccountType = 0
	AccountTypeUser  AccountType = 1
	AccountTypeAgent AccountType = 2
)

type Account struct {
	AccountID   string
	Identifier  string
	AccountType AccountType
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (a Account) Clone() Account {
	return a
}

type Profile struct {
	AccountID     string
	DisplayName   string
	Name          string
	Gender        string
	BirthDate     string
	Region        string
	AvatarMediaID string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (p Profile) Clone() Profile {
	return p
}

type User struct {
	AccountID      string
	UserID         string
	Identifier     string
	DisplayName    string
	Name           string
	Gender         string
	BirthDate      string
	Region         string
	AccountType    AccountType
	AccountTypeSet bool
	AvatarMediaID  string
	CreatedAt      time.Time // V0 compatibility alias for ProfileCreatedAt.
	UpdatedAt      time.Time // V0 compatibility alias for ProfileUpdatedAt.

	AccountCreatedAt time.Time
	AccountUpdatedAt time.Time
	ProfileCreatedAt time.Time
	ProfileUpdatedAt time.Time
}

func (u User) Clone() User {
	u.normalizeAliases()
	return u
}

func (u *User) normalizeAliases() {
	if u.AccountID == "" {
		u.AccountID = u.UserID
	}
	if u.UserID == "" {
		u.UserID = u.AccountID
	}
	if u.ProfileCreatedAt.IsZero() {
		u.ProfileCreatedAt = u.CreatedAt
	}
	if u.ProfileUpdatedAt.IsZero() {
		u.ProfileUpdatedAt = u.UpdatedAt
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = u.ProfileCreatedAt
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = u.ProfileUpdatedAt
	}
	if u.AccountCreatedAt.IsZero() {
		u.AccountCreatedAt = u.CreatedAt
	}
	if u.AccountUpdatedAt.IsZero() {
		u.AccountUpdatedAt = u.UpdatedAt
	}
}

func NewAccountProfile(account Account, profile Profile) User {
	user := User{
		AccountID:        account.AccountID,
		UserID:           account.AccountID,
		Identifier:       account.Identifier,
		DisplayName:      profile.DisplayName,
		Name:             profile.Name,
		Gender:           profile.Gender,
		BirthDate:        profile.BirthDate,
		Region:           profile.Region,
		AccountType:      account.AccountType,
		AccountTypeSet:   true,
		AvatarMediaID:    profile.AvatarMediaID,
		CreatedAt:        profile.CreatedAt,
		UpdatedAt:        profile.UpdatedAt,
		AccountCreatedAt: account.CreatedAt,
		AccountUpdatedAt: account.UpdatedAt,
		ProfileCreatedAt: profile.CreatedAt,
		ProfileUpdatedAt: profile.UpdatedAt,
	}
	user.normalizeAliases()
	return user
}

func (u User) ToAccount() Account {
	u.normalizeAliases()
	return Account{
		AccountID:   u.AccountID,
		Identifier:  u.Identifier,
		AccountType: u.AccountType,
		CreatedAt:   u.AccountCreatedAt,
		UpdatedAt:   u.AccountUpdatedAt,
	}
}

func (u User) ToProfile() Profile {
	u.normalizeAliases()
	return Profile{
		AccountID:     u.AccountID,
		DisplayName:   u.DisplayName,
		Name:          u.Name,
		Gender:        u.Gender,
		BirthDate:     u.BirthDate,
		Region:        u.Region,
		AvatarMediaID: u.AvatarMediaID,
		CreatedAt:     u.ProfileCreatedAt,
		UpdatedAt:     u.ProfileUpdatedAt,
	}
}

func NormalizeAccountType(value AccountType) (AccountType, bool) {
	switch value {
	case AccountTypeUser, AccountTypeAgent, AccountTypeAdmin:
		return value, true
	default:
		return AccountTypeUser, false
	}
}

func DefaultAccountTypeIfUnset(value AccountType) AccountType {
	if value == 0 {
		return AccountTypeUser
	}
	return value
}

func (t AccountType) IsValid() bool {
	switch t {
	case AccountTypeUser, AccountTypeAgent, AccountTypeAdmin:
		return true
	default:
		return false
	}
}
