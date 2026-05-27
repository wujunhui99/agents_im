package handler

import (
	"net/http"
	"strings"
	"time"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/service/message/api/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func authenticatedRoutes(serverCtx *svc.ServiceContext, routes []rest.Route) []rest.Route {
	if serverCtx == nil || serverCtx.ActiveSessionRepository() == nil {
		return routes
	}
	return rest.WithMiddleware(activeSessionMiddleware(serverCtx), routes...)
}

func activeSessionMiddleware(serverCtx *svc.ServiceContext) rest.Middleware {
	auth := serverCtx.AuthConfig()
	activeSessions := serverCtx.ActiveSessionRepository()
	tokenManager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, err := tokenManager.Validate(bearerToken(r.Header.Get("Authorization")))
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if err := authrepo.ValidateActiveSession(r.Context(), activeSessions, claims); err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			next(w, r)
		}
	}
}

func bearerToken(headerValue string) string {
	headerValue = strings.TrimSpace(headerValue)
	if headerValue == "" {
		return ""
	}
	parts := strings.Fields(headerValue)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
