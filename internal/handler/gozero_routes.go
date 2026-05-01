package handler

import (
	"net/http"

	agenthandler "github.com/wujunhui99/agents_im/internal/handler/agent"
	friendshandler "github.com/wujunhui99/agents_im/internal/handler/friends"
	groupshandler "github.com/wujunhui99/agents_im/internal/handler/groups"
	messagehandler "github.com/wujunhui99/agents_im/internal/handler/message"
	userhandler "github.com/wujunhui99/agents_im/internal/handler/user"
	"github.com/wujunhui99/agents_im/internal/health"
	"github.com/wujunhui99/agents_im/internal/observability"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "agents-im-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("user_logic", serverCtx != nil && serverCtx.UserLogic != nil, "configured"),
			componentCheck("friends_logic", serverCtx != nil && serverCtx.FriendsLogic != nil, "configured"),
			componentCheck("message_logic", serverCtx != nil && serverCtx.MessageLogic != nil, "configured"),
			componentCheck("repository", serverCtx != nil && serverCtx.Repo != nil, "configured"),
			componentCheck("message_repository", serverCtx != nil && serverCtx.MessageRepo != nil, "configured"),
		}
	})
	addUserRoutes(server, serverCtx)
	addFriendsRoutes(server, serverCtx)
	addMessageRoutes(server, serverCtx)
}

func RegisterUserGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "user-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("user_logic", serverCtx != nil && serverCtx.UserLogic != nil, "configured"),
			componentCheck("repository", serverCtx != nil && serverCtx.Repo != nil, "configured"),
		}
	})
	addUserRoutes(server, serverCtx)
}

func RegisterFriendsGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "friends-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("friends_logic", serverCtx != nil && serverCtx.FriendsLogic != nil, "configured"),
			componentCheck("repository", serverCtx != nil && serverCtx.Repo != nil, "configured"),
		}
	})
	addFriendsRoutes(server, serverCtx)
}

func RegisterGroupsGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "groups-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("groups_logic", serverCtx != nil && serverCtx.GroupsLogic != nil, "configured"),
			componentCheck("groups_repository", serverCtx != nil && serverCtx.GroupsRepo != nil, "configured"),
		}
	})
	addGroupsRoutes(server, serverCtx)
}

func RegisterMessageGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "message-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("message_logic", serverCtx != nil && serverCtx.MessageLogic != nil, "configured"),
			componentCheck("message_repository", serverCtx != nil && serverCtx.MessageRepo != nil, "configured"),
			componentCheck("outbox_repository", serverCtx != nil && serverCtx.OutboxRepo != nil, "configured"),
		}
	})
	addMessageRoutes(server, serverCtx)
}

func RegisterAgentGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroObservabilityHandlers(server, "agent-api", func(*http.Request) []health.Check {
		return []health.Check{
			componentCheck("auth_config", serverCtx != nil && serverCtx.Auth.AccessSecret != "", "configured"),
			componentCheck("agent_logic", serverCtx != nil && serverCtx.AgentLogic != nil, "configured"),
			componentCheck("agent_repository", serverCtx != nil && serverCtx.AgentRepo != nil, "configured"),
		}
	})
	addAgentRoutes(server, serverCtx)
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

func addUserRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
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
	}, jwtOption(serverCtx))

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/users",
			Handler: userhandler.CreateUserHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/users/exists",
			Handler: userhandler.ExistsUserHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/users/:identifier",
			Handler: userhandler.GetUserByIdentifierHandler(serverCtx),
		},
	})
}

func addFriendsRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
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
			Method:  http.MethodDelete,
			Path:    "/friends/:user_id",
			Handler: friendshandler.DeleteFriendHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/friends/:user_id",
			Handler: friendshandler.GetFriendshipHandler(serverCtx),
		},
	}, jwtOption(serverCtx))
}

func addGroupsRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/groups",
			Handler: groupshandler.CreateGroupHandler(serverCtx),
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
			Method:  http.MethodGet,
			Path:    "/groups/:group_id",
			Handler: groupshandler.GetGroupHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/groups/:group_id/members",
			Handler: groupshandler.ListMembersHandler(serverCtx),
		},
	}, jwtOption(serverCtx))
}

func addMessageRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/messages",
			Handler: messagehandler.SendMessageHandler(serverCtx),
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
	}, jwtOption(serverCtx))
}

func addAgentRoutes(server *rest.Server, serverCtx *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
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
	}, jwtOption(serverCtx))
}

func jwtOption(serverCtx *svc.ServiceContext) rest.RouteOption {
	return rest.WithJwt(serverCtx.Auth.AccessSecret)
}
