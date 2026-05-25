package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	authrepo "github.com/wujunhui99/agents_im/internal/auth/repository"
	"github.com/wujunhui99/agents_im/internal/auth/token"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	adminhandler "github.com/wujunhui99/agents_im/internal/handler/admin"
	agenthandler "github.com/wujunhui99/agents_im/internal/handler/agent"
	authhandler "github.com/wujunhui99/agents_im/internal/handler/auth"
	friendshandler "github.com/wujunhui99/agents_im/internal/handler/friends"
	groupshandler "github.com/wujunhui99/agents_im/internal/handler/groups"
	mediahandler "github.com/wujunhui99/agents_im/internal/handler/media"
	messagehandler "github.com/wujunhui99/agents_im/internal/handler/message"
	userhandler "github.com/wujunhui99/agents_im/internal/handler/user"
	"github.com/wujunhui99/agents_im/internal/health"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/observability"
	adminsvc "github.com/wujunhui99/agents_im/internal/servicecontext/admin"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
	friendssvc "github.com/wujunhui99/agents_im/internal/servicecontext/friends"
	groupssvc "github.com/wujunhui99/agents_im/internal/servicecontext/groups"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type authRouteContext interface {
	AuthConfig() config.JWTAuthConfig
	ActiveSessionRepository() authrepo.ActiveSessionRepository
}

func RegisterAuthGoZeroHandlers(server *rest.Server, serverCtx *authsvc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "auth-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_logic", serverCtx != nil && serverCtx.AuthLogic != nil, "configured"),
			componentCheck("auth_repository", serverCtx != nil && serverCtx.AuthRepo != nil, "configured"),
			componentCheck("user_client", serverCtx != nil && serverCtx.Users != nil, "configured"),
			componentCheck("mail_rpc_client", serverCtx != nil && serverCtx.Mailer != nil, "configured"),
		}
	})

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/auth/register/email-code",
			Handler: authhandler.RequestRegistrationEmailCodeHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/auth/login",
			Handler: authhandler.LoginHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/auth/register",
			Handler: authhandler.RegisterHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/auth/validate",
			Handler: authhandler.ValidateTokenHandler(serverCtx),
		},
	})
}

func RegisterUserGoZeroHandlers(server *rest.Server, serverCtx *usersvc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "user-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("account_logic", serverCtx != nil && serverCtx.AccountLogic != nil, "configured"),
			componentCheck("user_logic", serverCtx != nil && serverCtx.UserLogic != nil, "configured"),
			componentCheck("repository", serverCtx != nil && serverCtx.Repo != nil, "configured"),
			componentCheck("media_logic", serverCtx != nil && serverCtx.MediaLogic != nil, "configured"),
		}
	})
	addUserRoutes(server, serverCtx)
	addMediaRoutes(server, serverCtx)
}

func RegisterFriendsGoZeroHandlers(server *rest.Server, serverCtx *friendssvc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "friends-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("friends_logic", serverCtx != nil && serverCtx.FriendsLogic != nil, "configured"),
			componentCheck("repository", serverCtx != nil && serverCtx.Repo != nil, "configured"),
		}
	})
	addFriendsRoutes(server, serverCtx)
}

func RegisterGroupsGoZeroHandlers(server *rest.Server, serverCtx *groupssvc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "groups-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("groups_logic", serverCtx != nil && serverCtx.GroupsLogic != nil, "configured"),
			componentCheck("groups_repository", serverCtx != nil && serverCtx.GroupsRepo != nil, "configured"),
		}
	})
	addGroupsRoutes(server, serverCtx)
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

func RegisterAgentGoZeroHandlers(server *rest.Server, serverCtx *agentsvc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "agent-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("agent_logic", serverCtx != nil && serverCtx.AgentLogic != nil, "configured"),
			componentCheck("agent_definition_logic", serverCtx != nil && serverCtx.AgentDefinitionLogic != nil, "configured"),
			componentCheck("agent_repository", serverCtx != nil && serverCtx.AgentRepo != nil, "configured"),
			componentCheck("agent_registry_repository", serverCtx != nil && serverCtx.AgentRegistryRepo != nil, "configured"),
		}
	})
	addAgentRoutes(server, serverCtx)
}

func RegisterAdminGoZeroHandlers(server *rest.Server, serverCtx *adminsvc.ServiceContext) {
	addAdminRoutes(server, serverCtx)
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

func addUserRoutes(server *rest.Server, serverCtx *usersvc.ServiceContext) {
	server.AddRoutes(authenticatedRoutes(serverCtx, []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/me",
			Handler: userhandler.GetMeHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/me",
			Handler: userhandler.UpdateMeHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/me/avatar",
			Handler: userhandler.UpdateMeAvatarHandler(serverCtx),
		},
	}), jwtOption(serverCtx))

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/users",
			Handler: userhandler.CreateUserHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/accounts",
			Handler: userhandler.CreateAccountHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/users/exists",
			Handler: userhandler.ExistsUserHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/accounts/exists",
			Handler: userhandler.ExistsAccountHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/users/:identifier",
			Handler: userhandler.GetUserByIdentifierHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/accounts/:identifier",
			Handler: userhandler.GetAccountByIdentifierHandler(serverCtx),
		},
	})
}

func addMediaRoutes(server *rest.Server, serverCtx *usersvc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/media/avatars/:media_id",
			Handler: mediahandler.GetAvatarHandler(serverCtx),
		},
	})

	server.AddRoutes(authenticatedRoutes(serverCtx, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/media/uploads",
			Handler: mediahandler.CreateUploadIntentHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/media/uploads/:media_id/complete",
			Handler: mediahandler.CompleteUploadHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/media/:media_id/download-url",
			Handler: mediahandler.GetDownloadURLHandler(serverCtx),
		},
	}), jwtOption(serverCtx))
}

func addFriendsRoutes(server *rest.Server, serverCtx *friendssvc.ServiceContext) {
	server.AddRoutes(authenticatedRoutes(serverCtx, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/friends",
			Handler: friendshandler.AddFriendHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/friends",
			Handler: friendshandler.ListFriendsHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/friends/requests",
			Handler: friendshandler.ListFriendRequestsHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/friends/requests/:user_id/accept",
			Handler: friendshandler.AcceptFriendRequestHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/friends/requests/:user_id/reject",
			Handler: friendshandler.RejectFriendRequestHandler(serverCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/friends/:user_id",
			Handler: friendshandler.DeleteFriendHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/friends/:user_id",
			Handler: friendshandler.GetFriendshipHandler(serverCtx),
		},
	}), jwtOption(serverCtx))
}

func addGroupsRoutes(server *rest.Server, serverCtx *groupssvc.ServiceContext) {
	server.AddRoutes(authenticatedRoutes(serverCtx, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/groups",
			Handler: groupshandler.CreateGroupHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/groups",
			Handler: groupshandler.ListGroupsHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/groups/:group_id/members",
			Handler: groupshandler.AddMemberHandler(serverCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/groups/:group_id/members/me",
			Handler: groupshandler.LeaveGroupHandler(serverCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/groups/:group_id/members/:user_id",
			Handler: groupshandler.KickMemberHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/groups/:group_id",
			Handler: groupshandler.GetGroupHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/groups/:group_id",
			Handler: groupshandler.UpdateGroupHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/groups/:group_id/members",
			Handler: groupshandler.ListMembersHandler(serverCtx),
		},
	}), jwtOption(serverCtx))
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

func addAgentRoutes(server *rest.Server, serverCtx *agentsvc.ServiceContext) {
	server.AddRoutes(authenticatedRoutes(serverCtx, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/agents",
			Handler: agenthandler.CreateAgentHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/agents",
			Handler: agenthandler.ListAgentsHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/agents/:agent_id",
			Handler: agenthandler.GetAgentHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/agents/:agent_id/definition",
			Handler: agenthandler.GetAgentDefinitionHandler(serverCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/agents/:agent_id/definition",
			Handler: agenthandler.UpdateAgentDefinitionHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/agents/:agent_id",
			Handler: agenthandler.UpdateAgentHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/agents/:agent_id/status",
			Handler: agenthandler.UpdateAgentStatusHandler(serverCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/agents/:agent_id",
			Handler: agenthandler.DeleteAgentHandler(serverCtx),
		},
	}), jwtOption(serverCtx))
}

func addAdminRoutes(server *rest.Server, serverCtx *adminsvc.ServiceContext) {
	routes := []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/admin/dashboard",
			Handler: adminhandler.DashboardHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/llm-traces",
			Handler: adminhandler.ListLLMTracesHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/llm-traces/:trace_id",
			Handler: adminhandler.GetLLMTraceHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/admin/feedback",
			Handler: adminhandler.ListFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/admin/feedback/:feedback_id",
			Handler: adminhandler.GetFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodPatch,
			Path:    "/api/admin/feedback/:feedback_id",
			Handler: adminhandler.UpdateFeedbackHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/conversations/:conversation_id/messages",
			Handler: adminhandler.GetConversationMessagesHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users",
			Handler: adminhandler.SearchUsersHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users/:account_id",
			Handler: adminhandler.GetUserHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users/:account_id/friends",
			Handler: adminhandler.GetUserFriendsHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/admin/users/:account_id/conversations",
			Handler: adminhandler.GetUserConversationsHandler(serverCtx),
		},
	}
	routes = authenticatedRoutes(serverCtx, routes)
	routes = rest.WithMiddleware(adminOnlyMiddleware(serverCtx), routes...)
	server.AddRoutes(routes, jwtOption(serverCtx))
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

func adminOnlyMiddleware(serverCtx *adminsvc.ServiceContext) rest.Middleware {
	auth := config.DefaultJWTAuthConfig()
	if serverCtx != nil {
		auth = serverCtx.AuthConfig()
	}
	tokenManager := token.NewHMACTokenManager(auth.AccessSecret, time.Duration(auth.AccessExpire)*time.Second)
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if serverCtx == nil || serverCtx.Accounts == nil {
				httpx.ErrorCtx(r.Context(), w, apperror.Internal("admin account repository is not configured"))
				return
			}
			userID, err := ctxuser.UserID(r.Context())
			if err != nil {
				claims, tokenErr := tokenManager.Validate(bearerToken(r.Header.Get("Authorization")))
				if tokenErr != nil {
					httpx.ErrorCtx(r.Context(), w, tokenErr)
					return
				}
				userID = claims.UserID
			}
			account, err := serverCtx.Accounts.GetByID(r.Context(), userID)
			if err != nil {
				httpx.ErrorCtx(r.Context(), w, err)
				return
			}
			if account.AccountType != model.AccountTypeAdmin {
				httpx.ErrorCtx(r.Context(), w, apperror.Forbidden("admin account is required"))
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
