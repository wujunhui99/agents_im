package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	authhandler "github.com/wujunhui99/agents_im/internal/auth/handler"
	authsvc "github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func newUserGoZeroRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()
	return newGoZeroRouter(t, func(server *rest.Server) {
		handler.RegisterUserGoZeroHandlers(server, serviceContext)
	})
}

func newFriendsGoZeroRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()
	return newGoZeroRouter(t, func(server *rest.Server) {
		handler.RegisterFriendsGoZeroHandlers(server, serviceContext)
	})
}

func newGroupsGoZeroRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()
	return newGoZeroRouter(t, func(server *rest.Server) {
		handler.RegisterGroupsGoZeroHandlers(server, serviceContext)
	})
}

func newMessageGoZeroRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()
	return newGoZeroRouter(t, func(server *rest.Server) {
		handler.RegisterMessageGoZeroHandlers(server, serviceContext)
	})
}

func newAgentGoZeroRouter(t *testing.T, serviceContext *svc.ServiceContext) http.Handler {
	t.Helper()
	return newGoZeroRouter(t, func(server *rest.Server) {
		handler.RegisterAgentGoZeroHandlers(server, serviceContext)
	})
}

func newAuthGoZeroRouter(t *testing.T, serviceContext *authsvc.ServiceContext) http.Handler {
	t.Helper()
	return newGoZeroRouter(t, func(server *rest.Server) {
		authhandler.RegisterGoZeroHandlers(server, serviceContext)
	})
}

func newGoZeroRouter(t *testing.T, register func(*rest.Server)) http.Handler {
	t.Helper()

	httpx.SetErrorHandler(response.GoZeroErrorHandler)
	server := rest.MustNewServer(rest.RestConf{
		ServiceConf: service.ServiceConf{Name: "test-api"},
		Host:        "127.0.0.1",
		Port:        8888,
	}, rest.WithUnauthorizedCallback(response.GoZeroUnauthorizedCallback))
	t.Cleanup(server.Stop)
	register(server)

	serverless, err := rest.NewServerless(server)
	if err != nil {
		t.Fatalf("build go-zero test router: %v", err)
	}
	return http.HandlerFunc(serverless.Serve)
}

func newJSONRequest(method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
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

func setRejectedLegacyXUserIDHeader(t *testing.T, req *http.Request, userID string) {
	t.Helper()

	req.Header.Set("X-User-Id", userID) // legacy X-User-Id rejection helper
}
