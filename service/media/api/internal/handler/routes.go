package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterHandlers(server *rest.Server, serverCtx *svc.ServiceContext) {
	// Public avatar display (browser <img>, no auth) -> redirect to presigned URL.
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/media/avatars/:media_id",
			Handler: GetAvatarHandler(serverCtx),
		},
	})

	// Authenticated upload/download lifecycle; owner is taken from the JWT.
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/media/uploads",
			Handler: CreateUploadIntentHandler(serverCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/media/uploads/:media_id/complete",
			Handler: CompleteUploadHandler(serverCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/media/:media_id/download-url",
			Handler: GetDownloadURLHandler(serverCtx),
		},
	}, rest.WithJwt(serverCtx.Config.Auth.AccessSecret))
}
