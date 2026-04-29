package handler

import (
	"net/http"

	authhandler "github.com/wujunhui99/agents_im/internal/auth/handler/auth"
	"github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/health"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: healthHandler("auth-api"),
		},
		{
			Method: http.MethodGet,
			Path:   "/readyz",
			Handler: health.ReadinessHandler("auth-api", func(*http.Request) []health.Check {
				return []health.Check{
					componentCheck("auth_logic", serverCtx != nil && serverCtx.AuthLogic != nil),
					componentCheck("credential_repository", serverCtx != nil && serverCtx.AuthRepo != nil),
					componentCheck("user_client", serverCtx != nil && serverCtx.Users != nil),
				}
			}),
		},
		{
			Method:  http.MethodGet,
			Path:    "/metrics",
			Handler: observability.MetricsHandler(),
		},
	})

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/auth/register",
			Handler: authhandler.RegisterHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/auth/login",
			Handler: authhandler.LoginHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/auth/validate",
			Handler: authhandler.ValidateTokenHandler(serverCtx),
		},
	})
}

func componentCheck(name string, ok bool) health.Check {
	if ok {
		return health.ComponentCheck(name, true, "configured")
	}
	return health.ComponentCheck(name, false, "missing")
}
