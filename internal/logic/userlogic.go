package logic

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/model"
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

func NewUserLogic(repo repository.UserRepository) *UserLogic {
	return &UserLogic{repo: repo}
}

type AccountLogic = UserLogic

func NewAccountLogic(repo repository.AccountRepository) *AccountLogic {
	return NewUserLogic(repo)
}

type UserProfile struct {
	UserID      string `json:"user_id"`
	Identifier  string `json:"identifier"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Age         int32  `json:"age"`
	Region      string `json:"region"`
	AccountType string `json:"account_type"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AccountProfile = UserProfile

type CreateUserRequest struct {
	Identifier  string `json:"identifier"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	Age         int32  `json:"age"`
	Region      string `json:"region"`
	AccountType string `json:"account_type"`
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

type UpdateUserProfileRequest struct {
	UserID      string  `json:"user_id"`
	DisplayName *string `json:"display_name,omitempty"`
	Name        *string `json:"name,omitempty"`
	Gender      *string `json:"gender,omitempty"`
	Age         *int32  `json:"age,omitempty"`
	Region      *string `json:"region,omitempty"`
}

type UpdateAccountProfileRequest = UpdateUserProfileRequest

func (l *UserLogic) CreateUser(ctx context.Context, req CreateUserRequest) (UserProfile, error) {
	identifier, err := NormalizeIdentifier(req.Identifier)
	if err != nil {
		return UserProfile{}, err
	}

	displayName, name, err := normalizeNames(req.DisplayName, req.Name, identifier)
	if err != nil {
		return UserProfile{}, err
	}

	gender, err := normalizeGender(req.Gender)
	if err != nil {
		return UserProfile{}, err
	}

	if err := validateAge(req.Age); err != nil {
		return UserProfile{}, err
	}

	region, err := normalizeRegion(req.Region)
	if err != nil {
		return UserProfile{}, err
	}

	accountType, ok := model.NormalizeAccountType(req.AccountType)
	if !ok {
		return UserProfile{}, apperror.InvalidArgument("account_type must be user, agent, or admin")
	}

	user, err := l.repo.Create(ctx, model.User{
		Identifier:  identifier,
		DisplayName: displayName,
		Name:        name,
		Gender:      gender,
		Age:         req.Age,
		Region:      region,
		AccountType: accountType,
	})
	if err != nil {
		return UserProfile{}, err
	}

	return toProfile(user), nil
}

func (l *UserLogic) CreateAccount(ctx context.Context, req CreateAccountRequest) (AccountProfile, error) {
	return l.CreateUser(ctx, req)
}

func (l *UserLogic) GetUserByIdentifier(ctx context.Context, req GetUserByIdentifierRequest) (UserProfile, error) {
	identifier, err := NormalizeIdentifier(req.Identifier)
	if err != nil {
		return UserProfile{}, err
	}

	user, err := l.repo.GetByIdentifier(ctx, identifier)
	if err != nil {
		return UserProfile{}, err
	}

	return toProfile(user), nil
}

func (l *UserLogic) GetAccountByIdentifier(ctx context.Context, req GetAccountByIdentifierRequest) (AccountProfile, error) {
	return l.GetUserByIdentifier(ctx, req)
}

func (l *UserLogic) ExistsByIdentifier(ctx context.Context, req ExistsByIdentifierRequest) (ExistsByIdentifierResponse, error) {
	identifier, err := NormalizeIdentifier(req.Identifier)
	if err != nil {
		return ExistsByIdentifierResponse{}, err
	}

	exists, err := l.repo.ExistsByIdentifier(ctx, identifier)
	if err != nil {
		return ExistsByIdentifierResponse{}, err
	}

	return ExistsByIdentifierResponse{Identifier: identifier, Exists: exists}, nil
}

func (l *UserLogic) GetUserByID(ctx context.Context, req GetUserByIDRequest) (UserProfile, error) {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return UserProfile{}, apperror.InvalidArgument("user_id is required")
	}

	user, err := l.repo.GetByID(ctx, userID)
	if err != nil {
		return UserProfile{}, err
	}

	return toProfile(user), nil
}

func (l *UserLogic) GetAccountByID(ctx context.Context, req GetAccountByIDRequest) (AccountProfile, error) {
	return l.GetUserByID(ctx, req)
}

func (l *UserLogic) UpdateUserProfile(ctx context.Context, req UpdateUserProfileRequest) (UserProfile, error) {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return UserProfile{}, apperror.InvalidArgument("user_id is required")
	}

	patch := repository.ProfilePatch{}
	if req.DisplayName != nil {
		value, err := normalizeProfileName(*req.DisplayName, "display_name")
		if err != nil {
			return UserProfile{}, err
		}
		patch.DisplayName = &value
		if req.Name == nil {
			patch.Name = &value
		}
	}
	if req.Name != nil {
		value, err := normalizeProfileName(*req.Name, "name")
		if err != nil {
			return UserProfile{}, err
		}
		patch.Name = &value
		if req.DisplayName == nil {
			patch.DisplayName = &value
		}
	}
	if req.Gender != nil {
		value, err := normalizeGender(*req.Gender)
		if err != nil {
			return UserProfile{}, err
		}
		patch.Gender = &value
	}
	if req.Age != nil {
		if err := validateAge(*req.Age); err != nil {
			return UserProfile{}, err
		}
		patch.Age = req.Age
	}
	if req.Region != nil {
		value, err := normalizeRegion(*req.Region)
		if err != nil {
			return UserProfile{}, err
		}
		patch.Region = &value
	}

	user, err := l.repo.UpdateProfile(ctx, userID, patch)
	if err != nil {
		return UserProfile{}, err
	}

	return toProfile(user), nil
}

func (l *UserLogic) UpdateAccountProfile(ctx context.Context, req UpdateAccountProfileRequest) (AccountProfile, error) {
	return l.UpdateUserProfile(ctx, req)
}

func NormalizeIdentifier(identifier string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(identifier))
	if len(normalized) < 3 || len(normalized) > 32 {
		return "", apperror.InvalidArgument("identifier must be 3 to 32 characters")
	}

	for idx, r := range normalized {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'
		if idx == 0 && !isLetter && !isDigit {
			return "", apperror.InvalidArgument("identifier must start with a letter or digit")
		}
		if !isLetter && !isDigit && !isUnderscore {
			return "", apperror.InvalidArgument("identifier can only contain letters, digits, and underscore")
		}
	}

	return normalized, nil
}

func normalizeNames(displayName string, name string, fallback string) (string, string, error) {
	displayName = strings.TrimSpace(displayName)
	name = strings.TrimSpace(name)

	if displayName == "" && name == "" {
		displayName = fallback
		name = fallback
	}
	if displayName == "" {
		displayName = name
	}
	if name == "" {
		name = displayName
	}

	displayName, err := normalizeProfileName(displayName, "display_name")
	if err != nil {
		return "", "", err
	}
	name, err = normalizeProfileName(name, "name")
	if err != nil {
		return "", "", err
	}

	return displayName, name, nil
}

func normalizeProfileName(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " cannot be empty")
	}
	if len([]rune(value)) > 64 {
		return "", apperror.InvalidArgument(field + " must be 64 characters or fewer")
	}
	return value, nil
}

func normalizeGender(gender string) (string, error) {
	gender = strings.ToLower(strings.TrimSpace(gender))
	if gender == "" {
		return GenderUnknown, nil
	}

	switch gender {
	case GenderUnknown, GenderMale, GenderFemale, GenderOther:
		return gender, nil
	default:
		return "", apperror.InvalidArgument("gender must be unknown, male, female, or other")
	}
}

func validateAge(age int32) error {
	if age < 0 || age > 150 {
		return apperror.InvalidArgument("age must be between 0 and 150")
	}
	return nil
}

func normalizeRegion(region string) (string, error) {
	region = strings.TrimSpace(region)
	if len([]rune(region)) > 128 {
		return "", apperror.InvalidArgument("region must be 128 characters or fewer")
	}
	return region, nil
}

func toProfile(user model.User) UserProfile {
	return UserProfile{
		UserID:      user.UserID,
		Identifier:  user.Identifier,
		DisplayName: user.DisplayName,
		Name:        user.Name,
		Gender:      user.Gender,
		Age:         user.Age,
		Region:      user.Region,
		AccountType: string(user.AccountType),
		CreatedAt:   formatTime(user.CreatedAt),
		UpdatedAt:   formatTime(user.UpdatedAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
