package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/pkg/auth/token"
	"github.com/wujunhui99/agents_im/pkg/config"
)

func newJSONRequest(method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func performJSON(handler http.Handler, method string, target string, body string) *httptest.ResponseRecorder {
	req := newJSONRequest(method, target, body)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func testJWTAuthConfig() config.JWTAuthConfig {
	return config.JWTAuthConfig{
		AccessSecret: "test-jwt-secret-change-me",
		AccessExpire: 3600,
	}
}

func bearerTokenForUser(t *testing.T, userID string) string {
	t.Helper()

	auth := testJWTAuthConfig()
	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	rawToken, _, err := manager.Issue(userID, userID, "", "")
	if err != nil {
		t.Fatalf("issue test jwt: %v", err)
	}
	return "Bearer " + rawToken
}

func mustCreateUser(t *testing.T, userLogic *logic.UserLogic, identifier string) logic.UserProfile {
	t.Helper()

	return mustCreateUserWithName(t, userLogic, identifier, "")
}

func mustCreateUserWithName(t *testing.T, userLogic *logic.UserLogic, identifier string, displayName string) logic.UserProfile {
	t.Helper()

	user, err := userLogic.CreateUser(context.Background(), logic.CreateUserRequest{Identifier: identifier, DisplayName: displayName})
	if err != nil {
		t.Fatalf("create user %q: %v", identifier, err)
	}
	return user
}

type envelope[T any] struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func decodeEnvelope[T any](t *testing.T, raw []byte, dst *envelope[T]) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if err := decoder.Decode(dst); err != nil {
		t.Fatalf("decode envelope: %v; body=%s", err, string(raw))
	}
}

func assertNumericSnowflakeID(t *testing.T, id string) {
	t.Helper()

	if strings.HasPrefix(id, "usr_") || strings.HasPrefix(id, "agt_") || strings.HasPrefix(id, "grp_") {
		t.Fatalf("id %q must not use legacy prefixes", id)
	}
	if len(id) < 15 || len(id) > 20 {
		t.Fatalf("id %q length = %d, want Snowflake numeric string length 15..20", id, len(id))
	}
	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		t.Fatalf("id %q is not a numeric Snowflake string: %v", id, err)
	}
	if parsed == 0 {
		t.Fatalf("id %q must be non-zero", id)
	}
}
