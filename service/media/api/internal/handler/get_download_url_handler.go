package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// GetDownloadURLHandler returns a presigned download URL for media the
// authenticated user is allowed to access (owner or message-attachment access).
func GetDownloadURLHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetMediaDownloadURLReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		userID, err := ctxuser.UserID(r.Context())
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.MediaRPC.GetDownloadURL(r.Context(), &mediaclient.GetDownloadURLRequest{
			OwnerUserId:     userID,
			RequesterUserId: userID,
			MediaId:         req.MediaID,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, &types.GetMediaDownloadURLResp{
			Code:    string(apperror.CodeOK),
			Message: "ok",
			Data: types.GetMediaDownloadURLData{
				MediaID:     resp.GetMediaId(),
				DownloadURL: resp.GetDownloadUrl(),
				ExpiresAt:   resp.GetExpiresAt(),
			},
		})
	}
}
