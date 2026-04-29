package handler

import (
	"net/http"

	authhandler "github.com/wujunhui99/agents_im/internal/auth/handler/auth"
	"github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/healthz",
		Handler: healthHandler,
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
