package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/internal/svc"
)

func TestUserLogicCreateDuplicateExistsAndUpdate(t *testing.T) {
	userLogic := logic.NewUserLogic(repository.NewMemoryRepository())
	ctx := context.Background()

	created, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  "Alice_001",
		DisplayName: "Alice",
		Gender:      "female",
		Age:         30,
		Region:      "Shanghai",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if created.Identifier != "alice_001" {
		t.Fatalf("identifier was not normalized: %q", created.Identifier)
	}

	_, err = userLogic.CreateUser(ctx, logic.CreateUserRequest{Identifier: "alice_001"})
	if err == nil || apperror.From(err).Code != apperror.CodeAlreadyExists {
		t.Fatalf("duplicate identifier error = %v, want ALREADY_EXISTS", err)
	}

	exists, err := userLogic.ExistsByIdentifier(ctx, logic.ExistsByIdentifierRequest{Identifier: "ALICE_001"})
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists.Exists {
		t.Fatal("created identifier should exist")
	}

	displayName := "Alice Updated"
	age := int32(31)
	updated, err := userLogic.UpdateUserProfile(ctx, logic.UpdateUserProfileRequest{
		UserID:      created.UserID,
		DisplayName: &displayName,
		Age:         &age,
	})
	if err != nil {
		t.Fatalf("update profile: %v", err)
	}
	if updated.DisplayName != "Alice Updated" || updated.Name != "Alice Updated" || updated.Age != 31 {
		t.Fatalf("unexpected updated profile: %+v", updated)
	}
	if updated.Identifier != created.Identifier || updated.UserID != created.UserID {
		t.Fatalf("immutable fields changed: before=%+v after=%+v", created, updated)
	}
}

func TestUserHTTPHandlers(t *testing.T) {
	serviceContext := svc.NewServiceContext(repository.NewMemoryRepository())
	mux := http.NewServeMux()
	handler.RegisterHandlers(mux, serviceContext)

	createBody := `{"identifier":"bob_001","display_name":"Bob","gender":"male","age":28,"region":"Beijing"}`
	createResp := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(createBody))
	mux.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNoSecretFields(t, createResp.Body.String())

	var created envelope[logic.UserProfile]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)
	if created.Data.UserID == "" {
		t.Fatal("created user_id is empty")
	}

	duplicateResp := httptest.NewRecorder()
	duplicateReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"identifier":"BOB_001"}`))
	mux.ServeHTTP(duplicateResp, duplicateReq)
	if duplicateResp.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicateResp.Code, duplicateResp.Body.String())
	}

	existsResp := httptest.NewRecorder()
	existsReq := httptest.NewRequest(http.MethodGet, "/users/exists?identifier=BOB_001", nil)
	mux.ServeHTTP(existsResp, existsReq)
	if existsResp.Code != http.StatusOK {
		t.Fatalf("exists status = %d, body = %s", existsResp.Code, existsResp.Body.String())
	}
	var exists envelope[logic.ExistsByIdentifierResponse]
	decodeEnvelope(t, existsResp.Body.Bytes(), &exists)
	if !exists.Data.Exists || exists.Data.Identifier != "bob_001" {
		t.Fatalf("unexpected exists response: %+v", exists.Data)
	}

	meWithoutHeaderResp := httptest.NewRecorder()
	meWithoutHeaderReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	mux.ServeHTTP(meWithoutHeaderResp, meWithoutHeaderReq)
	if meWithoutHeaderResp.Code != http.StatusUnauthorized {
		t.Fatalf("missing X-User-Id status = %d", meWithoutHeaderResp.Code)
	}

	meResp := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/me", nil)
	meReq.Header.Set("X-User-Id", created.Data.UserID)
	mux.ServeHTTP(meResp, meReq)
	if meResp.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meResp.Code, meResp.Body.String())
	}
	assertNoSecretFields(t, meResp.Body.String())

	patchResp := httptest.NewRecorder()
	patchReq := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"name":"Bobby","region":"Hangzhou"}`))
	patchReq.Header.Set("X-User-Id", created.Data.UserID)
	mux.ServeHTTP(patchResp, patchReq)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", patchResp.Code, patchResp.Body.String())
	}
	var patched envelope[logic.UserProfile]
	decodeEnvelope(t, patchResp.Body.Bytes(), &patched)
	if patched.Data.DisplayName != "Bobby" || patched.Data.Name != "Bobby" || patched.Data.Region != "Hangzhou" {
		t.Fatalf("unexpected patched profile: %+v", patched.Data)
	}

	publicResp := httptest.NewRecorder()
	publicReq := httptest.NewRequest(http.MethodGet, "/users/bob_001", nil)
	mux.ServeHTTP(publicResp, publicReq)
	if publicResp.Code != http.StatusOK {
		t.Fatalf("public status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	assertNoSecretFields(t, publicResp.Body.String())
}

func TestPatchRejectsImmutableFields(t *testing.T) {
	serviceContext := svc.NewServiceContext(repository.NewMemoryRepository())
	mux := http.NewServeMux()
	handler.RegisterHandlers(mux, serviceContext)

	createResp := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"identifier":"carol_001"}`))
	mux.ServeHTTP(createResp, createReq)
	var created envelope[logic.UserProfile]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)

	patchResp := httptest.NewRecorder()
	identifierReq := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"identifier":"changed"}`))
	identifierReq.Header.Set("X-User-Id", created.Data.UserID)
	mux.ServeHTTP(patchResp, identifierReq)
	if patchResp.Code != http.StatusBadRequest {
		t.Fatalf("immutable field patch status = %d, body = %s", patchResp.Code, patchResp.Body.String())
	}

	userIDResp := httptest.NewRecorder()
	userIDReq := httptest.NewRequest(http.MethodPatch, "/me", strings.NewReader(`{"user_id":"usr_999999"}`))
	userIDReq.Header.Set("X-User-Id", created.Data.UserID)
	mux.ServeHTTP(userIDResp, userIDReq)
	if userIDResp.Code != http.StatusBadRequest {
		t.Fatalf("user_id patch status = %d, body = %s", userIDResp.Code, userIDResp.Body.String())
	}
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

func assertNoSecretFields(t *testing.T, body string) {
	t.Helper()
	lower := strings.ToLower(body)
	for _, forbidden := range []string{"password", "password_hash", "verification", "oauth", "credential"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("response leaked forbidden field %q: %s", forbidden, body)
		}
	}
}
