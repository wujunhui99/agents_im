package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	authlogic "github.com/wujunhui99/agents_im/internal/auth/logic"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	authsvc "github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/auth/useradapter"
	userlogic "github.com/wujunhui99/agents_im/internal/logic"
	userrepo "github.com/wujunhui99/agents_im/internal/repository"
	rootsvc "github.com/wujunhui99/agents_im/internal/svc"
)

func TestAuthLogicRegisterLoginAndValidateToken(t *testing.T) {
	userLogic := userlogic.NewUserLogic(userrepo.NewMemoryRepository())
	authLogic := newAuthLogic(userLogic)
	ctx := context.Background()

	registered, err := authLogic.Register(ctx, authlogic.RegisterRequest{
		Identifier:  "Alice_001",
		Password:    "correct-password",
		DisplayName: "Alice",
		Gender:      "female",
		Age:         30,
		Region:      "Shanghai",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if registered.UserID == "" || registered.Identifier != "alice_001" || registered.Token == "" || registered.ExpiresAt == "" {
		t.Fatalf("register response missing required auth fields")
	}
	assertLooksLikeJWT(t, registered.Token)

	if _, err := time.Parse(time.RFC3339, registered.ExpiresAt); err != nil {
		t.Fatalf("expires_at is not RFC3339: %v", err)
	}

	userProfile, err := userLogic.GetUserByID(ctx, userlogic.GetUserByIDRequest{UserID: registered.UserID})
	if err != nil {
		t.Fatalf("user profile was not created: %v", err)
	}
	rawProfile, err := json.Marshal(userProfile)
	if err != nil {
		t.Fatalf("marshal user profile: %v", err)
	}
	assertNoSecretFields(t, string(rawProfile))

	_, err = authLogic.Register(ctx, authlogic.RegisterRequest{
		Identifier: "ALICE_001",
		Password:   "another-password",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("duplicate register error = %v, want ALREADY_EXISTS", err)
	}

	loggedIn, err := authLogic.Login(ctx, authlogic.LoginRequest{
		Identifier: "ALICE_001",
		Password:   "correct-password",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if loggedIn.UserID != registered.UserID || loggedIn.Identifier != registered.Identifier || loggedIn.Token == "" {
		t.Fatalf("login response missing required auth fields")
	}
	assertLooksLikeJWT(t, loggedIn.Token)

	_, err = authLogic.Login(ctx, authlogic.LoginRequest{
		Identifier: "alice_001",
		Password:   "wrong-password",
	})
	if err == nil || apperror.From(err).Code != apperror.CodeUnauthenticated {
		t.Fatalf("wrong password error = %v, want UNAUTHENTICATED", err)
	}

	validated, err := authLogic.ValidateToken(ctx, authlogic.ValidateTokenRequest{Token: loggedIn.Token})
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if !validated.Valid || validated.UserID != registered.UserID || validated.Identifier != registered.Identifier {
		t.Fatalf("unexpected token validation response: %+v", validated)
	}
}

func TestAuthTokenExpires(t *testing.T) {
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	manager := token.NewHMACTokenManagerWithClock("test-secret", time.Second, func() time.Time {
		return now
	})

	rawToken, claims, err := manager.Issue("usr_000001", "alice_001")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if !claims.ExpiresAt.After(claims.IssuedAt) {
		t.Fatalf("token does not expire after issue time: %+v", claims)
	}

	now = now.Add(2 * time.Second)
	_, err = manager.Validate(rawToken)
	if err == nil || apperror.From(err).Code != apperror.CodeUnauthenticated {
		t.Fatalf("expired token error = %v, want UNAUTHENTICATED", err)
	}
}

func TestAuthHTTPHandlers(t *testing.T) {
	userLogic := userlogic.NewUserLogic(userrepo.NewMemoryRepository())
	serviceContext := authsvc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager("test-secret", time.Hour),
	)
	mux := newAuthGoZeroRouter(t, serviceContext)

	registerResp := httptest.NewRecorder()
	registerReq := newJSONRequest(http.MethodPost, "/auth/register", `{"identifier":"bob_001","password":"correct-password","display_name":"Bob"}`)
	mux.ServeHTTP(registerResp, registerReq)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}
	assertNoSecretFields(t, registerResp.Body.String())

	var registered envelope[authlogic.AuthResponse]
	decodeEnvelope(t, registerResp.Body.Bytes(), &registered)
	if registered.Data.UserID == "" || registered.Data.Token == "" {
		t.Fatalf("register response missing required auth fields")
	}
	assertLooksLikeJWT(t, registered.Data.Token)

	duplicateResp := httptest.NewRecorder()
	duplicateReq := newJSONRequest(http.MethodPost, "/auth/register", `{"identifier":"BOB_001","password":"another-password"}`)
	mux.ServeHTTP(duplicateResp, duplicateReq)
	if duplicateResp.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicateResp.Code, duplicateResp.Body.String())
	}

	loginResp := httptest.NewRecorder()
	loginReq := newJSONRequest(http.MethodPost, "/auth/login", `{"identifier":"BOB_001","password":"correct-password"}`)
	mux.ServeHTTP(loginResp, loginReq)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}
	assertNoSecretFields(t, loginResp.Body.String())

	var loggedIn envelope[authlogic.AuthResponse]
	decodeEnvelope(t, loginResp.Body.Bytes(), &loggedIn)
	assertLooksLikeJWT(t, loggedIn.Data.Token)

	wrongPasswordResp := httptest.NewRecorder()
	wrongPasswordReq := newJSONRequest(http.MethodPost, "/auth/login", `{"identifier":"bob_001","password":"wrong-password"}`)
	mux.ServeHTTP(wrongPasswordResp, wrongPasswordReq)
	if wrongPasswordResp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, body = %s", wrongPasswordResp.Code, wrongPasswordResp.Body.String())
	}

	validateResp := httptest.NewRecorder()
	validateReq := newJSONRequest(http.MethodPost, "/auth/validate", `{"token":"`+loggedIn.Data.Token+`"}`)
	mux.ServeHTTP(validateResp, validateReq)
	if validateResp.Code != http.StatusOK {
		t.Fatalf("validate status = %d, body = %s", validateResp.Code, validateResp.Body.String())
	}

	var validated envelope[authlogic.ValidateTokenResponse]
	decodeEnvelope(t, validateResp.Body.Bytes(), &validated)
	if !validated.Data.Valid || validated.Data.UserID != registered.Data.UserID || validated.Data.Identifier != "bob_001" {
		t.Fatalf("unexpected validate response: %+v", validated.Data)
	}
}

func TestAuthIssuedBearerTokenAccessesMe(t *testing.T) {
	authConfig := testJWTAuthConfig()
	repo := userrepo.NewMemoryRepository()
	userLogic := userlogic.NewUserLogic(repo)
	authServiceContext := authsvc.NewServiceContext(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		token.NewHMACTokenManager(authConfig.AccessSecret, time.Duration(authConfig.AccessExpire)*time.Second),
	)
	authMux := newAuthGoZeroRouter(t, authServiceContext)
	userMux := newUserGoZeroRouter(t, rootsvc.NewServiceContextWithAuth(repo, authConfig))

	registerResp := httptest.NewRecorder()
	registerReq := newJSONRequest(http.MethodPost, "/auth/register", `{"identifier":"bearer_me","password":"correct-password","display_name":"Bearer User"}`)
	authMux.ServeHTTP(registerResp, registerReq)
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status = %d", registerResp.Code)
	}

	var registered envelope[authlogic.AuthResponse]
	decodeEnvelope(t, registerResp.Body.Bytes(), &registered)
	assertLooksLikeJWT(t, registered.Data.Token)

	meResp := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+registered.Data.Token)
	userMux.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}

	var me envelope[userlogic.UserProfile]
	decodeEnvelope(t, meResp.Body.Bytes(), &me)
	if me.Data.UserID != registered.Data.UserID || me.Data.Identifier != "bearer_me" {
		t.Fatalf("unexpected /me user: %+v", me.Data)
	}
}

func newAuthLogic(userLogic *userlogic.UserLogic) *authlogic.AuthLogic {
	return authlogic.NewAuthLogic(
		authrepo.NewMemoryRepository(),
		useradapter.NewLogicClient(userLogic),
		authlogic.NewPasswordHasher(),
		token.NewHMACTokenManager("test-secret", time.Hour),
	)
}

func assertLooksLikeJWT(t *testing.T, raw string) {
	t.Helper()
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a JWT compact serialization")
	}
	for _, part := range parts {
		if part == "" {
			t.Fatalf("token has an empty JWT segment")
		}
	}
}
