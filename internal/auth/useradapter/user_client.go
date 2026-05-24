package useradapter

import (
	"context"
	"time"

	userlogic "github.com/wujunhui99/agents_im/internal/logic"
)

type ExistsResult struct {
	Identifier string
	Exists     bool
}

type CreateUserRequest struct {
	Identifier      string
	Email           string
	EmailVerifiedAt time.Time
	DisplayName     string
	Name            string
	Gender          string
	BirthDate       string
	Region          string
}

type UserProfile struct {
	UserID          string
	Identifier      string
	Email           string
	EmailVerifiedAt time.Time
	DisplayName     string
	Name            string
	Gender          string
	BirthDate       string
	Region          string
	AccountType     string
	AvatarMediaID   string
	AvatarURL       string
	CreatedAt       string
	UpdatedAt       string
}

type UserClient interface {
	ExistsByIdentifier(ctx context.Context, identifier string) (ExistsResult, error)
	CreateUser(ctx context.Context, req CreateUserRequest) (UserProfile, error)
	GetUserByID(ctx context.Context, userID string) (UserProfile, error)
}

type LogicClient struct {
	logic *userlogic.UserLogic
}

func NewLogicClient(logic *userlogic.UserLogic) *LogicClient {
	return &LogicClient{logic: logic}
}

func NormalizeIdentifier(identifier string) (string, error) {
	return userlogic.NormalizeIdentifier(identifier)
}

func (c *LogicClient) ExistsByIdentifier(ctx context.Context, identifier string) (ExistsResult, error) {
	result, err := c.logic.ExistsByIdentifier(ctx, userlogic.ExistsByIdentifierRequest{
		Identifier: identifier,
	})
	if err != nil {
		return ExistsResult{}, err
	}

	return ExistsResult{
		Identifier: result.Identifier,
		Exists:     result.Exists,
	}, nil
}

func (c *LogicClient) CreateUser(ctx context.Context, req CreateUserRequest) (UserProfile, error) {
	profile, err := c.logic.CreateUser(ctx, userlogic.CreateUserRequest{
		Identifier:      req.Identifier,
		Email:           req.Email,
		EmailVerifiedAt: req.EmailVerifiedAt,
		DisplayName:     req.DisplayName,
		Name:            req.Name,
		Gender:          req.Gender,
		BirthDate:       req.BirthDate,
		Region:          req.Region,
	})
	if err != nil {
		return UserProfile{}, err
	}

	return UserProfile{
		UserID:          profile.UserID,
		Identifier:      profile.Identifier,
		Email:           profile.Email,
		EmailVerifiedAt: profile.EmailVerifiedAt,
		DisplayName:     profile.DisplayName,
		Name:            profile.Name,
		Gender:          profile.Gender,
		BirthDate:       profile.BirthDate,
		Region:          profile.Region,
		AccountType:     profile.AccountType,
		AvatarMediaID:   profile.AvatarMediaID,
		AvatarURL:       profile.AvatarURL,
		CreatedAt:       profile.CreatedAt,
		UpdatedAt:       profile.UpdatedAt,
	}, nil
}

func (c *LogicClient) GetUserByID(ctx context.Context, userID string) (UserProfile, error) {
	profile, err := c.logic.GetUserByID(ctx, userlogic.GetUserByIDRequest{
		UserID: userID,
	})
	if err != nil {
		return UserProfile{}, err
	}

	return UserProfile{
		UserID:          profile.UserID,
		Identifier:      profile.Identifier,
		Email:           profile.Email,
		EmailVerifiedAt: profile.EmailVerifiedAt,
		DisplayName:     profile.DisplayName,
		Name:            profile.Name,
		Gender:          profile.Gender,
		BirthDate:       profile.BirthDate,
		Region:          profile.Region,
		AccountType:     profile.AccountType,
		AvatarMediaID:   profile.AvatarMediaID,
		AvatarURL:       profile.AvatarURL,
		CreatedAt:       profile.CreatedAt,
		UpdatedAt:       profile.UpdatedAt,
	}, nil
}
