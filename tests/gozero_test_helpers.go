package tests

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authhandler "github.com/wujunhui99/agents_im/internal/auth/handler"
	authsvc "github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/handler"
	"github.com/wujunhui99/agents_im/internal/response"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/router"
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
	})
	t.Cleanup(server.Stop)
	register(server)

	rt := router.NewRouter()
	for _, route := range server.Routes() {
		if err := rt.Handle(route.Method, route.Path, http.HandlerFunc(route.Handler)); err != nil {
			t.Fatalf("register go-zero test route %s %s: %v", route.Method, route.Path, err)
		}
	}
	return rt
}

func newJSONRequest(method string, target string, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
