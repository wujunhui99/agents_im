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

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/auth/token"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/agent/api/internal/svc"
	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/agent/rpc/agentclient"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc"
)

// TestAgentHTTPHandlers 验证 agent-api 纯 BFF（#606）：HTTP 路由经鉴权后转发 agent-rpc gRPC，
// 响应映射回 BFF 视图；账号类型等业务校验由 agent-rpc 负责（fake 用 status error 模拟，BFF 经
// rpcerror.FromStatus 还原 apperror → HTTP code）。
func TestAgentHTTPHandlers(t *testing.T) {
	fake := &fakeAgentRPC{}
	serviceContext := svc.NewServiceContextWithAuth(fake, testJWTAuthConfig())
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
	if fake.lastCreate.GetCreatedBy() != "usr_admin" || fake.lastCreate.GetImUserId() != "usr_agent" {
		t.Fatalf("BFF did not forward created_by/im_user_id: %+v", fake.lastCreate)
	}
	var created envelope[agentView]
	decodeEnvelope(t, createResp.Body.Bytes(), &created)
	if created.Data.AgentID != "ag_1" || created.Data.CreatedBy != "usr_admin" {
		t.Fatalf("unexpected created agent: %+v", created.Data)
	}

	statusResp := httptest.NewRecorder()
	statusReq := newJSONRequest(http.MethodPatch, "/agents/ag_1/status", `{"status":"active"}`)
	statusReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status update = %d, body = %s", statusResp.Code, statusResp.Body.String())
	}
	if fake.lastStatus.GetStatus() != "active" || fake.lastStatus.GetAgentId() != "ag_1" {
		t.Fatalf("BFF did not forward status update: %+v", fake.lastStatus)
	}

	forbiddenResp := httptest.NewRecorder()
	forbiddenReq := newJSONRequest(http.MethodPost, "/agents", `{"im_user_id":"usr_user","name":"Wrong Type Bot"}`)
	forbiddenReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(forbiddenResp, forbiddenReq)
	if forbiddenResp.Code != http.StatusForbidden {
		t.Fatalf("user-type account binding status = %d, body = %s", forbiddenResp.Code, forbiddenResp.Body.String())
	}

	deleteResp := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/agents/ag_1", nil)
	deleteReq.Header.Set("Authorization", adminBearer)
	mux.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete/archive status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	if fake.lastStatus.GetStatus() != "archived" {
		t.Fatalf("delete should archive via status update, got %q", fake.lastStatus.GetStatus())
	}
}

// --- fake agent-rpc client (BFF target) ---

type fakeAgentRPC struct {
	agentclient.Agent
	lastCreate *agentpb.CreateAgentRequest
	lastStatus *agentpb.UpdateAgentStatusRequest
}

func (f *fakeAgentRPC) CreateAgent(_ context.Context, in *agentpb.CreateAgentRequest, _ ...grpc.CallOption) (*agentpb.AgentResponse, error) {
	f.lastCreate = in
	if in.GetImUserId() == "usr_user" {
		return nil, rpcerror.ToStatus(apperror.Forbidden("account_type must be agent"))
	}
	return &agentpb.AgentResponse{Agent: &agentpb.AgentEntity{
		AgentId:   "ag_1",
		ImUserId:  in.GetImUserId(),
		Name:      in.GetName(),
		Status:    "disabled",
		CreatedBy: in.GetCreatedBy(),
	}}, nil
}

func (f *fakeAgentRPC) UpdateAgentStatus(_ context.Context, in *agentpb.UpdateAgentStatusRequest, _ ...grpc.CallOption) (*agentpb.AgentResponse, error) {
	f.lastStatus = in
	return &agentpb.AgentResponse{Agent: &agentpb.AgentEntity{AgentId: in.GetAgentId(), Status: in.GetStatus()}}, nil
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
	rawToken, _, err := manager.Issue(userID, userID, "", "")
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

type agentView struct {
	AgentID   string `json:"agent_id"`
	Status    string `json:"status"`
	CreatedBy string `json:"created_by"`
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
