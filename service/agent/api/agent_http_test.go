package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestAgentHTTPHandlers(t *testing.T) {
	serviceContext := svc.NewServiceContextWithAuth(
		repository.NewMemoryAgentRepository(),
		testAccountTypeChecker{accountTypes: map[string]string{
			"usr_agent": logic.AccountTypeAgent,
			"usr_user":  string(model.AccountTypeUser),
		}},
		testJWTAuthConfig(),
	)
	mux := newAgentAPIServiceRouter(t, serviceContext)
	adminBearer := bearerTokenForUser(t, "usr_admin")

	missingTokenResp := httptest.NewRecorder()
	missingTokenReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_agent","name":"Support Bot"}`)
	mux.ServeHTTP(missingTokenResp, missingTokenReq)
	if missingTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d", missingTokenResp.Code)
	}

	createResp := httptest.NewRecorder()
	createReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_agent","name":"Support Bot","description":"support"}`)
	createReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create agent status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNoSecretFields(t, createResp.Body.String())

	var created envelope[logic.AgentInfo]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)
	if created.Data.AgentID == "" || created.Data.CreatedBy != "usr_admin" {
		t.Fatalf("unexpected created agent: %+v", created.Data)
	}

	getResp := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/agents/"+created.Data.AgentID, nil)
	getReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("get agent status = %d, body = %s", getResp.Code, getResp.Body.String())
	}

	updateResp := httptest.NewRecorder()
	updateReq := newJSONRequest(http.MethodPatch, "/agents/"+created.Data.AgentID, `{"name":"Renamed Bot"}`)
	updateReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(updateResp, updateReq)
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update agent status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	statusResp := httptest.NewRecorder()
	statusReq := newJSONRequest(http.MethodPatch, "/agents/"+created.Data.AgentID+"/status", `{"status":"active"}`)
	statusReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status update = %d, body = %s", statusResp.Code, statusResp.Body.String())
	}
	var activated envelope[logic.AgentInfo]
	decodeEnvelope(t, statusResp.Body.Bytes(), &activated)
	if activated.Data.Status != logic.AgentStatusActive {
		t.Fatalf("unexpected activated agent: %+v", activated.Data)
	}

	listResp := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/agents?status=active", nil)
	listReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed envelope[logic.ListAgentsResponse]
	decodeEnvelope(t, listResp.Body.Bytes(), &listed)
	if len(listed.Data.Agents) != 1 || listed.Data.Agents[0].AgentID != created.Data.AgentID {
		t.Fatalf("unexpected list response: %+v", listed.Data.Agents)
	}

	forbiddenResp := httptest.NewRecorder()
	forbiddenReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_user","name":"Wrong Type Bot"}`)
	forbiddenReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("user-type account binding status = %d, body = %s", forbiddenResp.Code, forbiddenResp.Body.String())
	}

	deleteResp := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/agents/"+created.Data.AgentID, nil)
	deleteReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete/archive status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var archived envelope[logic.AgentInfo]
	decodeEnvelope(t, deleteResp.Body.Bytes(), &archived)
	if archived.Data.Status != logic.AgentStatusArchived {
		t.Fatalf("delete should archive agent: %+v", archived.Data)
	}
}

// --- self-contained test helpers (agent service in-process HTTP harness) ---

func newAgentAPIServiceRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()
	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "test-api"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	registerAgentAPIServiceHandlers(server, serviceContext)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build go-zero test router: %v", err)
	}
	return http.HandlerFunc(serverless.Serve)
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
	rawToken, _, err := manager.Issue(userID, userID)
	if err != nil {
		t.Fatalf("issue test jwt: %v", err)
	}
	return "Bearer " + rawToken
}

func newJSONRequest(method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
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

type testAccountTypeChecker struct {
	accountTypes map[string]string
}

func (c testAccountTypeChecker) EnsureUserAccountType(_ context.Context, userID string, accountType string) error {
	actual, exists := c.accountTypes[userID]
	if !exists {
		return apperror.NotFound("user not found")
	}
	if actual != accountType {
		return apperror.Forbidden("im_user_id must reference account_type=agent")
	}
	return nil
}
