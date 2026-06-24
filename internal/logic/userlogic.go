package logic

import (
	"time"

	"github.com/wujunhui99/agents_im/internal/repository"
)

const (
	GenderUnknown = "unknown"
	GenderMale    = "male"
	GenderFemale  = "female"
	GenderOther   = "other"
)

type UserLogic struct {
	repo repository.UserRepository
}

type AccountLogic = UserLogic

type UserProfile struct {
	AccountID       string    `json:"account_id"`
	UserID          string    `json:"user_id"`
	Identifier      string    `json:"identifier"`
	Email           string    `json:"email"`
	EmailVerifiedAt time.Time `json:"-"`
	DisplayName     string    `json:"display_name"`
	Name            string    `json:"name"`
	Gender          string    `json:"gender"`
	BirthDate       string    `json:"birth_date"`
	Region          string    `json:"region"`
	AccountType     string    `json:"account_type"`
	AvatarMediaID   string    `json:"avatar_media_id"`
	AvatarURL       string    `json:"avatar_url"`
	CreatedAt       string    `json:"created_at"`
	UpdatedAt       string    `json:"updated_at"`
}

type AccountProfile = UserProfile

type CreateUserRequest struct {
	Identifier      string    `json:"identifier"`
	Email           string    `json:"email"`
	EmailVerifiedAt time.Time `json:"-"`
	DisplayName     string    `json:"display_name"`
	Name            string    `json:"name"`
	Gender          string    `json:"gender"`
	BirthDate       string    `json:"birth_date"`
	Region          string    `json:"region"`
	AccountType     string    `json:"account_type"`
}

type CreateAccountRequest = CreateUserRequest

type GetUserByIdentifierRequest struct {
	Identifier string `json:"identifier"`
}

type GetAccountByIdentifierRequest = GetUserByIdentifierRequest

type ExistsByIdentifierRequest struct {
	Identifier string `json:"identifier"`
}

type ExistsByIdentifierResponse struct {
	Identifier string `json:"identifier"`
	Exists     bool   `json:"exists"`
}

type AccountExistsByIdentifierResponse = ExistsByIdentifierResponse

type GetUserByIDRequest struct {
	UserID string `json:"user_id"`
}

type GetAccountByIDRequest = GetUserByIDRequest

type GetUsersByIDsRequest struct {
	UserIDs []string `json:"user_ids"`
}

type UpdateUserProfileRequest struct {
	UserID      string  `json:"user_id"`
	DisplayName *string `json:"display_name,omitempty"`
	Name        *string `json:"name,omitempty"`
	Gender      *string `json:"gender,omitempty"`
	BirthDate   *string `json:"birth_date,omitempty"`
	Region      *string `json:"region,omitempty"`
}

type UpdateAccountProfileRequest = UpdateUserProfileRequest

type UpdateUserAvatarRequest struct {
	UserID  string `json:"user_id"`
	MediaID string `json:"media_id"`
}
