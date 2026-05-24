package logic

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/auth/mailadapter"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
)

func TestRegistrationEmailCodeRequestSendsMailAndRegisterConsumesCode(t *testing.T) {
	ctx := context.Background()
	users := newAuthProfileClient()
	repo := authrepo.NewMemoryRepository()
	mailer := &recordingRegistrationMailer{}
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	authLogic := NewAuthLogicWithOptions(
		repo,
		users,
		NewPasswordHasher(),
		token.NewHMACTokenManager("unit-test-secret", time.Hour),
		AuthOptions{
			Mailer:                    mailer,
			VerificationRepo:          repo,
			RegistrationCodeGenerator: fixedRegistrationCode("123456"),
			Clock:                     func() time.Time { return now },
		},
	)

	codeResp, err := authLogic.RequestRegistrationEmailCode(ctx, RegistrationEmailCodeRequest{
		Email: " Alice@Example.COM ",
	})
	if err != nil {
		t.Fatalf("request registration email code: %v", err)
	}
	if codeResp.Email != "alice@example.com" {
		t.Fatalf("normalized email = %q, want alice@example.com", codeResp.Email)
	}
	if codeResp.ExpireMinutes != 10 {
		t.Fatalf("expire_minutes = %d, want 10", codeResp.ExpireMinutes)
	}
	if len(mailer.requests) != 1 {
		t.Fatalf("mail sends = %d, want 1", len(mailer.requests))
	}
	mailReq := mailer.requests[0]
	if len(mailReq.Recipients) != 1 || mailReq.Recipients[0] != "alice@example.com" {
		t.Fatalf("mail recipients = %#v, want normalized email", mailReq.Recipients)
	}
	if mailReq.TemplateID != 177952 {
		t.Fatalf("template_id = %d, want 177952", mailReq.TemplateID)
	}
	if mailReq.TemplateData["code"] != "123456" || mailReq.TemplateData["expire_minutes"] != "10" {
		t.Fatalf("template data = %#v, want code and expire_minutes", mailReq.TemplateData)
	}
	if mailReq.Subject == "" {
		t.Fatalf("mail subject is empty; Tencent SES requires a non-empty Subject")
	}

	registered, err := authLogic.Register(ctx, RegisterRequest{
		Identifier:            "alice_email",
		Email:                 "ALICE@example.com",
		EmailVerificationCode: "123456",
		Password:              "test-password",
		DisplayName:           "Alice",
	})
	if err != nil {
		t.Fatalf("register with verification code: %v", err)
	}
	if registered.UserID == "" || registered.Identifier != "alice_email" {
		t.Fatalf("register response = %+v, want issued account", registered)
	}

	_, err = authLogic.Register(ctx, RegisterRequest{
		Identifier:            "alice_email_second",
		Email:                 "alice@example.com",
		EmailVerificationCode: "123456",
		Password:              "test-password",
		DisplayName:           "Alice Second",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("reuse verification code error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestRegistrationEmailCodeRequestFailsClosedWhenMailUnavailable(t *testing.T) {
	ctx := context.Background()
	users := newAuthProfileClient()
	repo := authrepo.NewMemoryRepository()
	authLogic := NewAuthLogicWithOptions(
		repo,
		users,
		NewPasswordHasher(),
		token.NewHMACTokenManager("unit-test-secret", time.Hour),
		AuthOptions{
			Mailer:                    &recordingRegistrationMailer{err: errors.New("rpc unavailable")},
			VerificationRepo:          repo,
			RegistrationCodeGenerator: fixedRegistrationCode("654321"),
			Clock:                     func() time.Time { return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC) },
		},
	)

	_, err := authLogic.RequestRegistrationEmailCode(ctx, RegistrationEmailCodeRequest{
		Email: "bob@example.com",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeServiceUnavailable {
		t.Fatalf("mail unavailable error = %v, want SERVICE_UNAVAILABLE", err)
	}

	_, err = authLogic.Register(ctx, RegisterRequest{
		Identifier:            "bob_email",
		Email:                 "bob@example.com",
		EmailVerificationCode: "654321",
		Password:              "test-password",
		DisplayName:           "Bob",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("register after failed send error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestRegistrationEmailCodeRequestRequiresMailerAndDoesNotPersistToken(t *testing.T) {
	ctx := context.Background()
	repo := authrepo.NewMemoryRepository()
	authLogic := NewAuthLogicWithOptions(
		repo,
		newAuthProfileClient(),
		NewPasswordHasher(),
		token.NewHMACTokenManager("unit-test-secret", time.Hour),
		AuthOptions{
			VerificationRepo:          repo,
			RegistrationCodeGenerator: fixedRegistrationCode("135790"),
			Clock:                     func() time.Time { return time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC) },
		},
	)

	_, err := authLogic.RequestRegistrationEmailCode(ctx, RegistrationEmailCodeRequest{
		Email: "missing-mailer@example.com",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeServiceUnavailable {
		t.Fatalf("missing mailer error = %v, want SERVICE_UNAVAILABLE", err)
	}

	_, err = repo.LatestEmailVerification(ctx, "register", "missing-mailer@example.com")
	if err == nil || apperror.From(err).Code != apperror.CodeNotFound {
		t.Fatalf("verification token after missing mailer = %v, want NOT_FOUND", err)
	}
}

func TestRegisterRejectsMissingWrongExpiredAndTooManyAttemptCodes(t *testing.T) {
	ctx := context.Background()
	users := newAuthProfileClient()
	repo := authrepo.NewMemoryRepository()
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	authLogic := NewAuthLogicWithOptions(
		repo,
		users,
		NewPasswordHasher(),
		token.NewHMACTokenManager("unit-test-secret", time.Hour),
		AuthOptions{
			Mailer:                    &recordingRegistrationMailer{},
			VerificationRepo:          repo,
			RegistrationCodeGenerator: fixedRegistrationCode("222333"),
			Clock:                     func() time.Time { return now },
			MaxVerificationAttempts:   2,
		},
	)

	_, err := authLogic.Register(ctx, RegisterRequest{
		Identifier: "missing_code",
		Email:      "missing-code@example.com",
		Password:   "test-password",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("missing code error = %v, want INVALID_ARGUMENT", err)
	}

	if _, err := authLogic.RequestRegistrationEmailCode(ctx, RegistrationEmailCodeRequest{Email: "wrong-code@example.com"}); err != nil {
		t.Fatalf("request wrong-code verification: %v", err)
	}
	_, err = authLogic.Register(ctx, RegisterRequest{
		Identifier:            "wrong_code",
		Email:                 "wrong-code@example.com",
		EmailVerificationCode: "000000",
		Password:              "test-password",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("wrong code error = %v, want INVALID_ARGUMENT", err)
	}
	_, err = authLogic.Register(ctx, RegisterRequest{
		Identifier:            "wrong_code_again",
		Email:                 "wrong-code@example.com",
		EmailVerificationCode: "111111",
		Password:              "test-password",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeRateLimited {
		t.Fatalf("too many attempts error = %v, want RATE_LIMITED", err)
	}

	if _, err := authLogic.RequestRegistrationEmailCode(ctx, RegistrationEmailCodeRequest{Email: "expired-code@example.com"}); err != nil {
		t.Fatalf("request expired-code verification: %v", err)
	}
	now = now.Add(11 * time.Minute)
	_, err = authLogic.Register(ctx, RegisterRequest{
		Identifier:            "expired_code",
		Email:                 "expired-code@example.com",
		EmailVerificationCode: "222333",
		Password:              "test-password",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("expired code error = %v, want INVALID_ARGUMENT", err)
	}
}

func TestLoginResponseIncludesDurableAvatarProfileFields(t *testing.T) {
	ctx := context.Background()
	users := newAuthProfileClient()
	repo := authrepo.NewMemoryRepository()
	authLogic := newVerifiedAuthLogic(repo, users, "avatar@example.com", "111222")

	registered, err := authLogic.Register(ctx, RegisterRequest{
		Identifier:            "alice_avatar",
		Email:                 "avatar@example.com",
		EmailVerificationCode: "111222",
		Password:              "test-password",
		DisplayName:           "Alice",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	users.profiles[registered.UserID] = useradapter.UserProfile{
		UserID:        registered.UserID,
		Identifier:    "alice_avatar",
		DisplayName:   "Alice Chen",
		Name:          "Alice Chen",
		Gender:        "female",
		BirthDate:     "1996-05-02",
		Region:        "Shanghai",
		AccountType:   "user",
		AvatarMediaID: "med_avatar_1",
		AvatarURL:     "/media/avatars/med_avatar_1",
	}

	loggedIn, err := authLogic.Login(ctx, LoginRequest{
		Identifier: "alice_avatar",
		Password:   "test-password",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if loggedIn.DisplayName != "Alice Chen" {
		t.Fatalf("display_name = %q, want Alice Chen", loggedIn.DisplayName)
	}
	if loggedIn.AvatarMediaID != "med_avatar_1" {
		t.Fatalf("avatar_media_id = %q, want med_avatar_1", loggedIn.AvatarMediaID)
	}
	if loggedIn.AvatarURL != "/media/avatars/med_avatar_1" {
		t.Fatalf("avatar_url = %q, want durable profile URL", loggedIn.AvatarURL)
	}
}

type authProfileClient struct {
	nextID       int
	identifierID map[string]string
	profiles     map[string]useradapter.UserProfile
}

func newAuthProfileClient() *authProfileClient {
	return &authProfileClient{
		nextID:       1000,
		identifierID: make(map[string]string),
		profiles:     make(map[string]useradapter.UserProfile),
	}
}

func (c *authProfileClient) ExistsByIdentifier(_ context.Context, identifier string) (useradapter.ExistsResult, error) {
	_, exists := c.identifierID[identifier]
	return useradapter.ExistsResult{Identifier: identifier, Exists: exists}, nil
}

func (c *authProfileClient) CreateUser(_ context.Context, req useradapter.CreateUserRequest) (useradapter.UserProfile, error) {
	c.nextID++
	userID := "auth_user_" + strconv.Itoa(c.nextID)
	profile := useradapter.UserProfile{
		UserID:          userID,
		Identifier:      req.Identifier,
		Email:           req.Email,
		EmailVerifiedAt: req.EmailVerifiedAt,
		DisplayName:     req.DisplayName,
		Name:            req.DisplayName,
		AccountType:     "user",
	}
	c.identifierID[req.Identifier] = userID
	c.profiles[userID] = profile
	return profile, nil
}

func (c *authProfileClient) GetUserByID(_ context.Context, userID string) (useradapter.UserProfile, error) {
	return c.profiles[userID], nil
}

type recordingRegistrationMailer struct {
	requests []mailadapter.SendTemplateEmailRequest
	err      error
}

func (m *recordingRegistrationMailer) SendTemplateEmail(_ context.Context, req mailadapter.SendTemplateEmailRequest) error {
	m.requests = append(m.requests, req)
	return m.err
}

func fixedRegistrationCode(code string) func() (string, error) {
	return func() (string, error) {
		return code, nil
	}
}

func newVerifiedAuthLogic(repo *authrepo.MemoryRepository, users useradapter.UserClient, email string, code string) *AuthLogic {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	authLogic := NewAuthLogicWithOptions(
		repo,
		users,
		NewPasswordHasher(),
		token.NewHMACTokenManager("unit-test-secret", time.Hour),
		AuthOptions{
			Mailer:                    &recordingRegistrationMailer{},
			VerificationRepo:          repo,
			RegistrationCodeGenerator: fixedRegistrationCode(code),
			Clock:                     func() time.Time { return now },
		},
	)
	if _, err := authLogic.RequestRegistrationEmailCode(context.Background(), RegistrationEmailCodeRequest{Email: email}); err != nil {
		panic(err)
	}
	return authLogic
}
