package handler

import (
	"net/http"

	friendshandler "github.com/wujunhui99/agents_im/internal/handler/friends"
	groupshandler "github.com/wujunhui99/agents_im/internal/handler/groups"
	messagehandler "github.com/wujunhui99/agents_im/internal/handler/message"
	userhandler "github.com/wujunhui99/agents_im/internal/handler/user"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroHealthHandler(server)
	addUserRoutes(server, serverCtx)
	addFriendsRoutes(server, serverCtx)
	addMessageRoutes(server, serverCtx)
}

func RegisterUserGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroHealthHandler(server)
	addUserRoutes(server, serverCtx)
}

func RegisterFriendsGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroHealthHandler(server)
	addFriendsRoutes(server, serverCtx)
}

func RegisterGroupsGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroHealthHandler(server)
	addGroupsRoutes(server, serverCtx)
}

func RegisterMessageGoZeroHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	registerGoZeroHealthHandler(server)
	addMessageRoutes(server, serverCtx)
}

func registerGoZeroHealthHandler(server *rest.Server) {
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/healthz",
		Handler: healthHandler,
	})
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
	}, jwtOption(serverCtx))

	server.AddRoutes([]rest.Route{
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
	})
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

func jwtOption(serverCtx *svc.ServiceContext) rest.RouteOption {
	return rest.WithJwt(serverCtx.Auth.AccessSecret)
}
