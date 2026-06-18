package logic

import (
	"context"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/middleware"
	"github.com/wujunhui99/agents_im/pkg/auth/token"
	"github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- fakes ---

// stubCredentials 在内存里实现 AuthCredentialsModel 的登录/注册查询，其余方法 nil-panic（未用）。
type stubCredentials struct {
	model.AuthCredentialsModel
	byIdentifier map[string]model.CredentialAuth
	emails       map[string]bool
	inserted     map[string]model.CredentialAuth
}

func newStubCredentials() *stubCredentials {
	return &stubCredentials{
		byIdentifier: map[string]model.CredentialAuth{},
		emails:       map[string]bool{},
		inserted:     map[string]model.CredentialAuth{},
	}
}

func (s *stubCredentials) FindAuthByIdentifier(_ context.Context, identifier string) (*model.CredentialAuth, error) {
	cred, ok := s.byIdentifier[identifier]
	if !ok {
		return nil, model.ErrNotFound
	}
	return &cred, nil
}

func (s *stubCredentials) EmailExists(_ context.Context, email string) (bool, error) {
	return s.emails[email], nil
}

func (s *stubCredentials) InsertCredential(_ context.Context, accountID string, passwordHash string, passwordAlgo int64) error {
	s.inserted[accountID] = model.CredentialAuth{AccountID: accountID, PasswordHash: passwordHash, PasswordAlgo: passwordAlgo}
	return nil
}

// stubEmailVerifications 在内存里实现验证码查询，其余方法 nil-panic（未用）。
type stubEmailVerifications struct {
	model.AuthEmailVerificationTokensModel
	latest    *model.AuthEmailVerificationTokens
	inserted  *model.AuthEmailVerificationTokens
	consumeAt time.Time
}

func (s *stubEmailVerifications) Latest(_ context.Context, _ int64, _ string) (*model.AuthEmailVerificationTokens, error) {
	if s.latest == nil {
		return nil, model.ErrNotFound
	}
	return s.latest, nil
}

func (s *stubEmailVerifications) SupersedeAndInsert(_ context.Context, data *model.AuthEmailVerificationTokens) error {
	s.inserted = data
	return nil
}

func (s *stubEmailVerifications) IncrementAttempts(_ context.Context, _ string, _ time.Time) (int64, error) {
	s.latest.AttemptCount++
	return s.latest.AttemptCount, nil
}

func (s *stubEmailVerifications) Consume(_ context.Context, _ string, now time.Time) (time.Time, error) {
	s.consumeAt = now
	return now, nil
}

// stubUserRPC 实现 userclient.User 的注册/登录三方法，其余 nil-panic（未用）。
type stubUserRPC struct {
	userclient.User
	exists     *userclient.ExistsByIdentifierResponse
	existsErr  error
	created    *userclient.UserResponse
	createErr  error
	byID       *userclient.UserResponse
	byIDErr    error
	createReqs []*userclient.CreateUserRequest
}

func (s *stubUserRPC) ExistsByIdentifier(_ context.Context, _ *userclient.ExistsByIdentifierRequest, _ ...grpc.CallOption) (*userclient.ExistsByIdentifierResponse, error) {
	return s.exists, s.existsErr
}

func (s *stubUserRPC) CreateUser(_ context.Context, in *userclient.CreateUserRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	s.createReqs = append(s.createReqs, in)
	return s.created, s.createErr
}

func (s *stubUserRPC) GetUserByID(_ context.Context, _ *userclient.GetUserByIDRequest, _ ...grpc.CallOption) (*userclient.UserResponse, error) {
	return s.byID, s.byIDErr
}

func newFlowSvc(creds model.AuthCredentialsModel, evs model.AuthEmailVerificationTokensModel, users userclient.User) *svc.ServiceContext {
	return &svc.ServiceContext{
		Tokens:             token.NewHMACTokenManager("test-secret", time.Hour),
		Sessions:           middleware.NewMemorySessionStore(),
		Users:              users,
		Credentials:        creds,
		EmailVerifications: evs,
	}
}

func mustBcrypt(t *testing.T, plain string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	return string(hash)
}

func codeOf(err error) codes.Code {
	return status.Code(err)
}

// --- Login ---

func TestLoginSuccessIssuesTokenWithProfile(t *testing.T) {
	creds := newStubCredentials()
	creds.byIdentifier["alice"] = model.CredentialAuth{AccountID: "usr_1", PasswordHash: mustBcrypt(t, "correct-horse"), PasswordAlgo: model.PasswordAlgoBcrypt}
	users := &stubUserRPC{byID: &userclient.UserResponse{User: &userclient.UserEntity{UserId: "usr_1", Identifier: "alice", DisplayName: "Alice", Email: "a@example.test"}}}
	l := NewLoginLogic(context.Background(), newFlowSvc(creds, &stubEmailVerifications{}, users))

	resp, err := l.Login(&auth.LoginRequest{Identifier: "Alice", Password: "correct-horse", Device: "web"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if resp.GetUserId() != "usr_1" || resp.GetDisplayName() != "Alice" || resp.GetEmail() != "a@example.test" {
		t.Fatalf("profile fields not hydrated from user-rpc: %+v", resp)
	}
	if resp.GetToken() == "" || resp.GetExpiresAt() == "" {
		t.Fatal("login did not issue a token")
	}
}

func TestLoginWrongPasswordUnauthenticated(t *testing.T) {
	creds := newStubCredentials()
	creds.byIdentifier["alice"] = model.CredentialAuth{AccountID: "usr_1", PasswordHash: mustBcrypt(t, "correct-horse"), PasswordAlgo: model.PasswordAlgoBcrypt}
	l := NewLoginLogic(context.Background(), newFlowSvc(creds, &stubEmailVerifications{}, &stubUserRPC{}))

	if _, err := l.Login(&auth.LoginRequest{Identifier: "alice", Password: "wrong-password"}); codeOf(err) != codes.Unauthenticated {
		t.Fatalf("wrong password code = %v, want Unauthenticated", codeOf(err))
	}
}

func TestLoginUnknownIdentifierUnauthenticated(t *testing.T) {
	l := NewLoginLogic(context.Background(), newFlowSvc(newStubCredentials(), &stubEmailVerifications{}, &stubUserRPC{}))

	if _, err := l.Login(&auth.LoginRequest{Identifier: "ghost", Password: "whatever-pw"}); codeOf(err) != codes.Unauthenticated {
		t.Fatalf("unknown identifier code = %v, want Unauthenticated", codeOf(err))
	}
}

// --- ValidateToken ---

func TestValidateTokenRoundTrip(t *testing.T) {
	svcCtx := newFlowSvc(newStubCredentials(), &stubEmailVerifications{}, &stubUserRPC{})
	raw, claims, err := svcCtx.Tokens.Issue("usr_1", "alice", "web", "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := svcCtx.Sessions.SetActive(context.Background(), claims.UserID, claims.Device, claims.JTI, time.Hour); err != nil {
		t.Fatalf("set active: %v", err)
	}

	resp, err := NewValidateTokenLogic(context.Background(), svcCtx).ValidateToken(&auth.ValidateTokenRequest{Token: raw})
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if !resp.GetValid() || resp.GetUserId() != "usr_1" || resp.GetIdentifier() != "alice" {
		t.Fatalf("unexpected validate response: %+v", resp)
	}
}

func TestValidateTokenRejectsInactiveSession(t *testing.T) {
	svcCtx := newFlowSvc(newStubCredentials(), &stubEmailVerifications{}, &stubUserRPC{})
	raw, _, err := svcCtx.Tokens.Issue("usr_1", "alice", "web", "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// 未 SetActive → 会话校验失败。
	if _, err := NewValidateTokenLogic(context.Background(), svcCtx).ValidateToken(&auth.ValidateTokenRequest{Token: raw}); err == nil {
		t.Fatal("validate without active session should fail")
	}
}

// --- Register ---

func validEmailToken(t *testing.T, email, code string) *model.AuthEmailVerificationTokens {
	t.Helper()
	return &model.AuthEmailVerificationTokens{
		Id:              "tok-1",
		Purpose:         model.EmailVerificationPurposeRegister,
		EmailNormalized: email,
		CodeHash:        mustBcrypt(t, code),
		CodeHashAlgo:    model.PasswordAlgoBcrypt,
		ExpiresAt:       time.Now().Add(10 * time.Minute),
		LastSentAt:      time.Now(),
	}
}

func TestRegisterSuccessCreatesUserAndCredential(t *testing.T) {
	creds := newStubCredentials()
	evs := &stubEmailVerifications{latest: validEmailToken(t, "a@example.test", "123456")}
	users := &stubUserRPC{
		exists:  &userclient.ExistsByIdentifierResponse{Exists: false, Identifier: "alice"},
		created: &userclient.UserResponse{User: &userclient.UserEntity{UserId: "usr_1", Identifier: "alice", Email: "a@example.test"}},
	}
	l := NewRegisterLogic(context.Background(), newFlowSvc(creds, evs, users))

	resp, err := l.Register(&auth.RegisterRequest{Identifier: "alice", Email: "a@example.test", EmailVerificationCode: "123456", Password: "correct-horse", Device: "web"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.GetUserId() != "usr_1" || resp.GetToken() == "" {
		t.Fatalf("register did not return authed session: %+v", resp)
	}
	cred, ok := creds.inserted["usr_1"]
	if !ok || cred.PasswordAlgo != model.PasswordAlgoBcrypt {
		t.Fatalf("credential not inserted with bcrypt: %+v ok=%v", cred, ok)
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte("correct-horse")) != nil {
		t.Fatal("inserted hash does not verify the password")
	}
	if len(users.createReqs) != 1 || users.createReqs[0].GetEmailVerifiedAt() == "" {
		t.Fatalf("CreateUser should carry email_verified_at: %+v", users.createReqs)
	}
}

func TestRegisterDuplicateIdentifierAlreadyExists(t *testing.T) {
	users := &stubUserRPC{exists: &userclient.ExistsByIdentifierResponse{Exists: true, Identifier: "alice"}}
	l := NewRegisterLogic(context.Background(), newFlowSvc(newStubCredentials(), &stubEmailVerifications{}, users))

	if _, err := l.Register(&auth.RegisterRequest{Identifier: "alice", Email: "a@example.test", EmailVerificationCode: "123456", Password: "correct-horse"}); codeOf(err) != codes.AlreadyExists {
		t.Fatalf("duplicate identifier code = %v, want AlreadyExists", codeOf(err))
	}
}

func TestRegisterWrongCodeInvalidArgument(t *testing.T) {
	evs := &stubEmailVerifications{latest: validEmailToken(t, "a@example.test", "123456")}
	users := &stubUserRPC{exists: &userclient.ExistsByIdentifierResponse{Exists: false, Identifier: "alice"}}
	l := NewRegisterLogic(context.Background(), newFlowSvc(newStubCredentials(), evs, users))

	if _, err := l.Register(&auth.RegisterRequest{Identifier: "alice", Email: "a@example.test", EmailVerificationCode: "000000", Password: "correct-horse"}); codeOf(err) != codes.InvalidArgument {
		t.Fatalf("wrong code result = %v, want InvalidArgument", codeOf(err))
	}
}
