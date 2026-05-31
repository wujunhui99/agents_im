package handler

import (
	"net/http"
	"strings"
	"time"

	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	messagehandler "github.com/wujunhui99/agents_im/internal/handler/message"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/pkg/config"
	"github.com/wujunhui99/agents_im/pkg/health"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type authRouteContext interface {
	AuthConfig() config.JWTAuthConfig
	ActiveSessionRepository() authrepo.ActiveSessionRepository
}

func RegisterMessageGoZeroHandlers(server *rest.Server, serverCtx *messagesvc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "message-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("message_logic", serverCtx != nil && serverCtx.MessageLogic != nil, "configured"),
			componentCheck("ai_hosting_logic", serverCtx != nil && serverCtx.AIHostingLogic != nil, "configured"),
			componentCheck("message_repository", serverCtx != nil && serverCtx.MessageRepo != nil, "configured"),
			componentCheck("feedback_logic", serverCtx != nil && serverCtx.FeedbackLogic != nil, "configured"),
			componentCheck("feedback_repository", serverCtx != nil && serverCtx.FeedbackRepo != nil, "configured"),
			componentCheck("ai_hosting_repository", serverCtx != nil && serverCtx.AIHostingRepo != nil, "configured"),
			componentCheck("outbox_repository", serverCtx != nil && serverCtx.OutboxRepo != nil, "configured"),
		}
	})
	addMessageRoutes(server, serverCtx)
}

func registerGoZeroObservabilityHandlers(server *rest.Server, service string, checks func(*http.Request) []health.Check) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/healthz",
			Handler: healthHandler(service),
		},
		{
			Method:  http.MethodGet,
			Path:    "/readyz",
			Handler: health.ReadinessHandler(service, checks),
		},
		{
			Method:  http.MethodGet,
			Path:    "/metrics",
			Handler: observability.MetricsHandler(),
		},
	})
}

func componentCheck(name string, ok bool, readyMessage string) health.Check {
	if ok {
		return health.ComponentCheck(name, true, readyMessage)
	}
	return health.ComponentCheck(name, false, "missing")
}

func addMessageRoutes(server *rest.Server, serverCtx *messagesvc.ServiceContext) {
	server.AddRoutes(authenticatedRoutes(serverCtx, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/messages",
			Handler: messagehandler.SendMessageHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/feedback",
			Handler: messagehandler.CreateFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/feedback",
			Handler: messagehandler.CreateFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/conversations/:conversation_id/messages",
			Handler: messagehandler.PullMessagesHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/conversations/seqs",
			Handler: messagehandler.GetConversationSeqsHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/conversations/:conversation_id/read",
			Handler: messagehandler.MarkConversationAsReadHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/conversations/:conversation_id/ai-hosting",
			Handler: messagehandler.GetConversationAIHostingHandler(serverCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/conversations/:conversation_id/ai-hosting",
			Handler: messagehandler.UpdateConversationAIHostingHandler(serverCtx),
		},
	}), jwtOption(serverCtx))
}

func jwtOption(serverCtx authRouteContext) rest.RouteOption {
	return rest.WithJwt(serverCtx.AuthConfig().AccessSecret)
}

func authenticatedRoutes(serverCtx authRouteContext, routes []rest.Route) []rest.Route {
	if serverCtx == nil || serverCtx.ActiveSessionRepository() == nil {
		return routes
	}
	return rest.WithMiddleware(activeSessionMiddleware(serverCtx), routes...)
}

func activeSessionMiddleware(serverCtx authRouteContext) rest.Middleware {
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
