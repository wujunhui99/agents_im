package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/common/share/auth/token"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/model"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestAdminRoutesRequireAuthenticatedAdminAccount(t *testing.T) {
	ctx := context.Background()
	auth := testAdminRouteJWTAuthConfig()
	accountRepo := repository.NewMemoryRepository()
	userLogic := logic.NewUserLogic(accountRepo)
	admin := mustCreateAdminRouteUser(t, ctx, userLogic, "admin_route_admin", model.AccountTypeAdmin)
	normal := mustCreateAdminRouteUser(t, ctx, userLogic, "admin_route_user", model.AccountTypeUser)
	serviceContext := svc.NewServiceContextWithAuth(svc.Dependencies{
		Accounts:    accountRepo,
		Friends:     accountRepo,
		Messages:    repository.NewMemoryMessageRepository(),
		AgentAudits: repository.NewMemoryAgentAuditRepository(),
		Feedback:    repository.NewMemoryFeedbackRepository(),
	}, auth)
	router := newAdminRouteTestRouter(t, serviceContext)

	unauthenticated := httptest.NewRecorder()
	router.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, body = %s", unauthenticated.Code, unauthenticated.Body.String())
	}

	nonAdmin := httptest.NewRecorder()
	nonAdminReq := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	nonAdminReq.Header.Set("Authorization", adminRouteBearerTokenForUser(t, auth, normal.UserID))
	router.ServeHTTP(nonAdmin, nonAdminReq)
	if nonAdmin.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, body = %s", nonAdmin.Code, nonAdmin.Body.String())
	}

	adminResp := httptest.NewRecorder()
	adminReq := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	adminReq.Header.Set("Authorization", adminRouteBearerTokenForUser(t, auth, admin.UserID))
	router.ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusOK {
		t.Fatalf("admin status = %d, body = %s", adminResp.Code, adminResp.Body.String())
	}
}

func TestAdminFeedbackJSONRouteUsesApiPrefix(t *testing.T) {
	ctx := context.Background()
	auth := testAdminRouteJWTAuthConfig()
	accountRepo := repository.NewMemoryRepository()
	feedbackRepo := repository.NewMemoryFeedbackRepository()
	userLogic := logic.NewUserLogic(accountRepo)
	admin := mustCreateAdminRouteUser(t, ctx, userLogic, "admin_route_feedback_admin", model.AccountTypeAdmin)
	created, err := feedbackRepo.CreateFeedback(ctx, model.Feedback{
		UserID:   admin.UserID,
		Category: model.FeedbackCategoryBug,
		Status:   model.FeedbackStatusNew,
		Title:    "broken feedback route",
		Content:  "admin feedback data must be JSON",
	})
	if err != nil {
		t.Fatalf("create feedback: %v", err)
	}
	serviceContext := svc.NewServiceContextWithAuth(svc.Dependencies{
		Accounts:    accountRepo,
		Friends:     accountRepo,
		Messages:    repository.NewMemoryMessageRepository(),
		AgentAudits: repository.NewMemoryAgentAuditRepository(),
		Feedback:    feedbackRepo,
	}, auth)
	router := newAdminRouteTestRouter(t, serviceContext)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/feedback?status=new", nil)
	req.Header.Set("Authorization", adminRouteBearerTokenForUser(t, auth, admin.UserID))
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("admin feedback status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("content type = %q, want JSON", contentType)
	}
	if strings.Contains(resp.Body.String(), "<!doctype html") {
		t.Fatalf("admin feedback returned HTML: %s", resp.Body.String())
	}

	var envelope struct {
		Code string `json:"code"`
		Data struct {
			Items []struct {
				FeedbackID string `json:"feedbackId"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode admin feedback JSON: %v; body = %s", err, resp.Body.String())
	}
	if envelope.Code != "OK" || len(envelope.Data.Items) != 1 || envelope.Data.Items[0].FeedbackID != created.FeedbackID {
		t.Fatalf("unexpected admin feedback envelope: %+v", envelope)
	}
}

func newAdminRouteTestRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "admin-route-test"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	registerAdminHandlers(server, serviceContext)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build admin route test router: %v", err)
	}
	return http.HandlerFunc(serverless.Serve)
}

func mustCreateAdminRouteUser(t *testing.T, ctx context.Context, userLogic *logic.UserLogic, identifier string, accountType model.AccountType) logic.UserProfile {
	t.Helper()

	user, err := userLogic.CreateUser(ctx, logic.CreateUserRequest{
		Identifier:  identifier,
		DisplayName: identifier,
		AccountType: string(accountType),
	})
	if err != nil {
		t.Fatalf("create user %q: %v", identifier, err)
	}
	return user
}

func testAdminRouteJWTAuthConfig() config.JWTAuthConfig {
	return config.JWTAuthConfig{
		AccessSecret: "admin-route-test-jwt-secret-change-me",
		AccessExpire: 3600,
	}
}

func adminRouteBearerTokenForUser(t *testing.T, auth config.JWTAuthConfig, userID string) string {
	t.Helper()

	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	rawToken, _, err := manager.Issue(userID, userID)
	if err != nil {
		t.Fatalf("issue admin route test jwt: %v", err)
	}
	return "Bearer " + rawToken
}
