package logic

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/model"
	"github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
)

type AuthLogic struct {
	repo   repository.CredentialRepository
	users  useradapter.UserClient
	hasher PasswordHasher
	tokens token.Manager
}

func NewAuthLogic(repo repository.CredentialRepository, users useradapter.UserClient, hasher PasswordHasher, tokens token.Manager) *AuthLogic {
	if hasher == nil {
		hasher = NewPasswordHasher()
	}
	return &AuthLogic{
		repo:   repo,
		users:  users,
		hasher: hasher,
		tokens: tokens,
	}
}

type RegisterRequest struct {
	Identifier  string `json:"identifier"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Name        string `json:"name"`
	Gender      string `json:"gender"`
	BirthDate   string `json:"birth_date"`
	Region      string `json:"region"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
	Token      string `json:"token"`
	ExpiresAt  string `json:"expires_at"`
}

type ValidateTokenResponse struct {
	Valid      bool   `json:"valid"`
	UserID     string `json:"user_id"`
	Identifier string `json:"identifier"`
	ExpiresAt  string `json:"expires_at"`
}

func (l *AuthLogic) Register(ctx context.Context, req RegisterRequest) (AuthResponse, error) {
	if err := validatePassword(req.Password); err != nil {
		return AuthResponse{}, err
	}

	exists, err := l.users.ExistsByIdentifier(ctx, req.Identifier)
	if err != nil {
		return AuthResponse{}, err
	}
	if exists.Exists {
		return AuthResponse{}, apperror.AlreadyExists("identifier already exists")
	}

	profile, err := l.users.CreateUser(ctx, useradapter.CreateUserRequest{
		Identifier:  exists.Identifier,
		DisplayName: req.DisplayName,
		Name:        req.Name,
		Gender:      req.Gender,
		BirthDate:   req.BirthDate,
		Region:      req.Region,
	})
	if err != nil {
		return AuthResponse{}, err
	}

	hash, salt, version, err := l.hasher.Hash(req.Password)
	if err != nil {
		return AuthResponse{}, apperror.Internal("password hash failed")
	}

	credential, err := l.repo.Create(ctx, model.Credential{
		Identifier:   profile.Identifier,
		UserID:       profile.UserID,
		PasswordHash: hash,
		Salt:         salt,
		HashVersion:  version,
	})
	if err != nil {
		return AuthResponse{}, err
	}

	return l.issueToken(ctx, credential.UserID, credential.Identifier)
}

func (l *AuthLogic) Login(ctx context.Context, req LoginRequest) (AuthResponse, error) {
	identifier, err := useradapter.NormalizeIdentifier(req.Identifier)
	if err != nil {
		return AuthResponse{}, err
	}
	if err := validatePassword(req.Password); err != nil {
		return AuthResponse{}, err
	}

	credential, err := l.repo.GetByIdentifier(ctx, identifier)
	if err != nil {
		return AuthResponse{}, apperror.Unauthenticated("invalid identifier or password")
	}
	if !l.hasher.Verify(req.Password, credential.Salt, credential.PasswordHash, credential.HashVersion) {
		return AuthResponse{}, apperror.Unauthenticated("invalid identifier or password")
	}

	return l.issueToken(ctx, credential.UserID, credential.Identifier)
}

func (l *AuthLogic) ValidateToken(ctx context.Context, req ValidateTokenRequest) (ValidateTokenResponse, error) {
	claims, err := l.tokens.Validate(req.Token)
	if err != nil {
		return ValidateTokenResponse{}, err
	}
	if err := repository.ValidateActiveSession(ctx, l.repo, claims); err != nil {
		return ValidateTokenResponse{}, err
	}

	return toValidateTokenResponse(claims), nil
}

func (l *AuthLogic) ParseToken(_ context.Context, req ValidateTokenRequest) (ValidateTokenResponse, error) {
	claims, err := l.tokens.Parse(req.Token)
	if err != nil {
		return ValidateTokenResponse{}, err
	}

	return toValidateTokenResponse(claims), nil
}

func (l *AuthLogic) issueToken(ctx context.Context, userID string, identifier string) (AuthResponse, error) {
	rawToken, claims, err := l.tokens.Issue(userID, identifier)
	if err != nil {
		return AuthResponse{}, err
	}
	if err := l.repo.SetActiveSession(ctx, model.ActiveSession{
		UserID:    claims.UserID,
		SessionID: claims.SessionID,
		IssuedAt:  claims.IssuedAt,
		ExpiresAt: claims.ExpiresAt,
	}); err != nil {
		return AuthResponse{}, err
	}

	return AuthResponse{
		UserID:     claims.UserID,
		Identifier: claims.Identifier,
		Token:      rawToken,
		ExpiresAt:  formatTime(claims.ExpiresAt),
	}, nil
}

func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return apperror.InvalidArgument("password is required")
	}
	length := len([]rune(password))
	if length < 8 || length > 128 {
		return apperror.InvalidArgument("password must be 8 to 128 characters")
	}
	return nil
}

func toValidateTokenResponse(claims token.Claims) ValidateTokenResponse {
	return ValidateTokenResponse{
		Valid:      true,
		UserID:     claims.UserID,
		Identifier: claims.Identifier,
		ExpiresAt:  formatTime(claims.ExpiresAt),
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
