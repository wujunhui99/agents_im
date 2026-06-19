package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/pkg/types"
	"github.com/wujunhui99/agents_im/service/media/api/internal/logic"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// GetDownloadURLHandler returns a presigned download URL after media-api(BFF)
// authorizes the download (uploader fast-path, else GetMessageRef link check +
// one-way friendship / group membership; EPIC #527 §4).
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
		resp, err := logic.GetDownloadURL(r.Context(), svcCtx, userID, req.MediaID, req.MsgID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, apiError(err))
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
