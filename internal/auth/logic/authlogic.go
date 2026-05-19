package logic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	stdmail "net/mail"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	"github.com/wujunhui99/agents_im/internal/auth/model"
	"github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	"github.com/zeromicro/go-zero/core/logx"
)

type AuthLogic struct {
	repo                    repository.CredentialRepository
	verificationRepo        repository.EmailVerificationRepository
	users                   useradapter.UserClient
	hasher                  PasswordHasher
	codeHasher              PasswordHasher
	tokens                  token.Manager
	mailer                  mailadapter.Client
	registrationCodeGen     func() (string, error)
	clock                   func() time.Time
	registrationCodeTTL     time.Duration
	registrationCooldown    time.Duration
	maxVerificationAttempts int
}

func NewAuthLogic(repo repository.CredentialRepository, users useradapter.UserClient, hasher PasswordHasher, tokens token.Manager) *AuthLogic {
	return NewAuthLogicWithOptions(repo, users, hasher, tokens, AuthOptions{})
}

type AuthOptions struct {
	VerificationRepo          repository.EmailVerificationRepository
	Mailer                    mailadapter.Client
	CodeHasher                PasswordHasher
	RegistrationCodeGenerator func() (string, error)
	Clock                     func() time.Time
	RegistrationCodeTTL       time.Duration
	RegistrationSendCooldown  time.Duration
	MaxVerificationAttempts   int
}

func NewAuthLogicWithOptions(repo repository.CredentialRepository, users useradapter.UserClient, hasher PasswordHasher, tokens token.Manager, opts AuthOptions) *AuthLogic {
	if hasher == nil {
		hasher = NewPasswordHasher()
	}
	codeHasher := opts.CodeHasher
	if codeHasher == nil {
		codeHasher = NewPasswordHasher()
	}
	codeGen := opts.RegistrationCodeGenerator
	if codeGen == nil {
		codeGen = generateNumericRegistrationCode
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	ttl := opts.RegistrationCodeTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	cooldown := opts.RegistrationSendCooldown
	if cooldown <= 0 {
		cooldown = time.Minute
	}
	maxAttempts := opts.MaxVerificationAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	return &AuthLogic{
		repo:                    repo,
		verificationRepo:        opts.VerificationRepo,
		users:                   users,
		hasher:                  hasher,
		codeHasher:              codeHasher,
		tokens:                  tokens,
		mailer:                  opts.Mailer,
		registrationCodeGen:     codeGen,
		clock:                   clock,
		registrationCodeTTL:     ttl,
		registrationCooldown:    cooldown,
		maxVerificationAttempts: maxAttempts,
	}
}

type RegisterRequest struct {
	Identifier            string `json:"identifier"`
	Email                 string `json:"email"`
	EmailVerificationCode string `json:"email_verification_code"`
	Password              string `json:"password"`
	DisplayName           string `json:"display_name"`
	Name                  string `json:"name"`
	Gender                string `json:"gender"`
	BirthDate             string `json:"birth_date"`
	Region                string `json:"region"`
}

type RegistrationEmailCodeRequest struct {
	Email string `json:"email"`
}

type RegistrationEmailCodeResponse struct {
	Email         string `json:"email"`
	ExpireMinutes int    `json:"expire_minutes"`
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type AuthResponse struct {
	UserID        string `json:"user_id"`
	Identifier    string `json:"identifier"`
	DisplayName   string `json:"display_name"`
	Name          string `json:"name"`
	Gender        string `json:"gender"`
	BirthDate     string `json:"birth_date"`
	Region        string `json:"region"`
	AccountType   string `json:"account_type"`
	AvatarMediaID string `json:"avatar_media_id"`
	AvatarURL     string `json:"avatar_url"`
	Token         string `json:"token"`
	ExpiresAt     string `json:"expires_at"`
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
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return AuthResponse{}, err
	}
	code := strings.TrimSpace(req.EmailVerificationCode)
	if code == "" {
		return AuthResponse{}, apperror.InvalidArgument("email verification code is required")
	}

	exists, err := l.users.ExistsByIdentifier(ctx, req.Identifier)
	if err != nil {
		return AuthResponse{}, err
	}
	if exists.Exists {
		return AuthResponse{}, apperror.AlreadyExists("identifier already exists")
	}
	verifiedAt, err := l.consumeRegistrationEmailCode(ctx, email, code)
	if err != nil {
		return AuthResponse{}, err
	}
	if err := l.ensureEmailAvailable(ctx, email); err != nil {
		return AuthResponse{}, err
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

	if _, err := l.repo.Create(ctx, model.Credential{
		Identifier:      profile.Identifier,
		UserID:          profile.UserID,
		Email:           email,
		EmailVerifiedAt: verifiedAt,
		PasswordHash:    hash,
		Salt:            salt,
		HashVersion:     version,
	}); err != nil {
		return AuthResponse{}, err
	}

	return l.issueToken(ctx, profile)
}

func (l *AuthLogic) RequestRegistrationEmailCode(ctx context.Context, req RegistrationEmailCodeRequest) (RegistrationEmailCodeResponse, error) {
	if l.verificationRepo == nil {
		return RegistrationEmailCodeResponse{}, apperror.Internal("email verification repository is required")
	}
	if l.mailer == nil {
		return RegistrationEmailCodeResponse{}, apperror.ServiceUnavailable("mail service is not configured")
	}
	email, err := normalizeEmail(req.Email)
	if err != nil {
		return RegistrationEmailCodeResponse{}, err
	}
	now := l.clock().UTC()
	if latest, err := l.verificationRepo.LatestEmailVerification(ctx, model.EmailVerificationPurposeRegister, email); err == nil {
		if latest.LastSentAt.After(now.Add(-l.registrationCooldown)) {
			return RegistrationEmailCodeResponse{}, apperror.RateLimited("registration email code was sent too recently")
		}
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return RegistrationEmailCodeResponse{}, err
	}

	code, err := l.registrationCodeGen()
	if err != nil {
		return RegistrationEmailCodeResponse{}, apperror.Internal("generate verification code failed")
	}
	if err := validateVerificationCodeFormat(code); err != nil {
		return RegistrationEmailCodeResponse{}, apperror.Internal("generate verification code failed")
	}
	codeHash, _, hashVersion, err := l.codeHasher.Hash(code)
	if err != nil {
		return RegistrationEmailCodeResponse{}, apperror.Internal("hash verification code failed")
	}
	tokenID, err := randomTokenID()
	if err != nil {
		return RegistrationEmailCodeResponse{}, apperror.Internal("generate verification token failed")
	}
	expireMinutes := int(l.registrationCodeTTL / time.Minute)
	if expireMinutes <= 0 {
		expireMinutes = 1
	}

	if err := l.mailer.SendTemplateEmail(ctx, mailadapter.SendTemplateEmailRequest{
		Recipients: []string{email},
		TemplateID: 177952,
		TemplateData: map[string]string{
			"code":           code,
			"expire_minutes": fmt.Sprintf("%d", expireMinutes),
		},
		Subject:        "AgenticIM 注册验证码",
		IdempotencyKey: "auth-register-email-code-" + tokenID,
	}); err != nil {
		logx.WithContext(ctx).Errorf("send registration verification email failed: %v", err)
		return RegistrationEmailCodeResponse{}, apperror.ServiceUnavailable("mail service is unavailable")
	}

	if _, err := l.verificationRepo.CreateEmailVerification(ctx, model.EmailVerificationToken{
		ID:              tokenID,
		Purpose:         model.EmailVerificationPurposeRegister,
		Email:           email,
		CodeHash:        codeHash,
		CodeHashVersion: hashVersion,
		ExpiresAt:       now.Add(l.registrationCodeTTL),
		LastSentAt:      now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		return RegistrationEmailCodeResponse{}, err
	}

	return RegistrationEmailCodeResponse{
		Email:         email,
		ExpireMinutes: expireMinutes,
	}, nil
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

	profile, err := l.users.GetUserByID(ctx, credential.UserID)
	if err != nil {
		return AuthResponse{}, err
	}

	return l.issueToken(ctx, profile)
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

func (l *AuthLogic) ensureEmailAvailable(ctx context.Context, email string) error {
	if l.repo == nil {
		return apperror.Internal("auth credential repository is required")
	}
	_, err := l.repo.GetByEmail(ctx, email)
	if err == nil {
		return apperror.AlreadyExists("email already exists")
	}
	if apperror.From(err).Code == apperror.CodeNotFound {
		return nil
	}
	return err
}

func (l *AuthLogic) consumeRegistrationEmailCode(ctx context.Context, email string, code string) (time.Time, error) {
	if l.verificationRepo == nil {
		return time.Time{}, apperror.Internal("email verification repository is required")
	}
	if err := validateVerificationCodeFormat(code); err != nil {
		return time.Time{}, err
	}
	now := l.clock().UTC()
	verification, err := l.verificationRepo.LatestEmailVerification(ctx, model.EmailVerificationPurposeRegister, email)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
		}
		return time.Time{}, err
	}
	if !verification.ConsumedAt.IsZero() || !now.Before(verification.ExpiresAt) {
		return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
	}
	if verification.AttemptCount >= l.maxVerificationAttempts {
		return time.Time{}, apperror.RateLimited("email verification attempts exceeded")
	}
	if !l.codeHasher.Verify(code, "", verification.CodeHash, verification.CodeHashVersion) {
		updated, err := l.verificationRepo.IncrementEmailVerificationAttempts(ctx, verification.ID, now)
		if err != nil {
			return time.Time{}, err
		}
		if updated.AttemptCount >= l.maxVerificationAttempts {
			return time.Time{}, apperror.RateLimited("email verification attempts exceeded")
		}
		return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
	}
	consumed, err := l.verificationRepo.ConsumeEmailVerification(ctx, verification.ID, now)
	if err != nil {
		return time.Time{}, err
	}
	return consumed.ConsumedAt, nil
}

func (l *AuthLogic) issueToken(ctx context.Context, profile useradapter.UserProfile) (AuthResponse, error) {
	rawToken, claims, err := l.tokens.Issue(profile.UserID, profile.Identifier)
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
		UserID:        claims.UserID,
		Identifier:    claims.Identifier,
		DisplayName:   profile.DisplayName,
		Name:          profile.Name,
		Gender:        profile.Gender,
		BirthDate:     profile.BirthDate,
		Region:        profile.Region,
		AccountType:   profile.AccountType,
		AvatarMediaID: profile.AvatarMediaID,
		AvatarURL:     profile.AvatarURL,
		Token:         rawToken,
		ExpiresAt:     formatTime(claims.ExpiresAt),
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

func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", apperror.InvalidArgument("email is required")
	}
	address, err := stdmail.ParseAddress(email)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		return "", apperror.InvalidArgument("email is invalid")
	}
	normalized := strings.ToLower(strings.TrimSpace(address.Address))
	if !strings.Contains(normalized, "@") {
		return "", apperror.InvalidArgument("email is invalid")
	}
	return normalized, nil
}

func validateVerificationCodeFormat(code string) error {
	if len(code) != 6 {
		return apperror.InvalidArgument("email verification code is invalid or expired")
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return apperror.InvalidArgument("email verification code is invalid or expired")
		}
	}
	return nil
}

func generateNumericRegistrationCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

func randomTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
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
