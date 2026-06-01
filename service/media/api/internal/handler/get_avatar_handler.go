package handler

import (
	"net/http"

	"github.com/wujunhui99/agents_im/common/share/types"
	"github.com/wujunhui99/agents_im/service/media/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/media/rpc/mediaclient"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// GetAvatarHandler serves public avatar display: redirect to a short-lived
// presigned object-storage URL. No auth (browser <img> requests carry none).
func GetAvatarHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetMediaDownloadURLReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		resp, err := svcCtx.MediaRPC.GetAvatarDisplayURL(r.Context(), &mediaclient.GetAvatarDisplayURLRequest{
			MediaId: req.MediaID,
		})
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, apiError(err))
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, resp.GetDownloadUrl(), http.StatusTemporaryRedirect)
	}
}
