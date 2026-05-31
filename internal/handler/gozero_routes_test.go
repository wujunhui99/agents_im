package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/repository"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/response"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestMessageFeedbackJSONRouteUsesApiPrefix(t *testing.T) {
	auth := testRouteJWTAuthConfig()
	feedbackRepo := repository.NewMemoryFeedbackRepository()
	serviceContext := messagesvc.NewServiceContextWithFeedback(
		repository.NewMemoryMessageRepository(),
		repository.NewMemoryMediaRepository(),
		feedbackRepo,
		nil,
		nil,
		auth,
	)
	router := newRouteTestMessageRouter(t, serviceContext)

	resp := httptest.NewRecorder()
	req := newRouteTestJSONRequest(http.MethodPost, "/api/feedback", `{"category":"bug","title":"feedback 405","content":"POST must not hit SPA static nginx"}`)
	req.Header.Set("Authorization", routeTestBearerTokenForUser(t, auth, "usr_feedback_route"))
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("feedback status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("content type = %q, want JSON", contentType)
	}

	var envelope struct {
		Code string `json:"code"`
		Data struct {
			FeedbackID string `json:"feedbackId"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode feedback JSON: %v; body = %s", err, resp.Body.String())
	}
	if envelope.Code != "OK" || envelope.Data.FeedbackID == "" || envelope.Data.Status != "new" {
		t.Fatalf("unexpected feedback envelope: %+v", envelope)
	}
}

func newRouteTestMessageRouter(t *testing.T, serviceContext *messagesvc.ServiceContext) http.Handler {
	t.Helper()

	httpx.SetErrorHandlerCtx(response.GoZeroErrorHandlerCtx)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "message-route-test"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	RegisterMessageGoZeroHandlers(server, serviceContext)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build message route test router: %v", err)
	}
	return http.HandlerFunc(serverless.Serve)
}

func testRouteJWTAuthConfig() config.JWTAuthConfig {
	return config.JWTAuthConfig{
		AccessSecret: "route-test-jwt-secret-change-me",
		AccessExpire: 3600,
	}
}

func routeTestBearerTokenForUser(t *testing.T, auth config.JWTAuthConfig, userID string) string {
	t.Helper()

	manager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	rawToken, _, err := manager.Issue(userID, userID)
	if err != nil {
		t.Fatalf("issue test jwt: %v", err)
	}
	return "Bearer " + rawToken
}

func newRouteTestJSONRequest(method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
