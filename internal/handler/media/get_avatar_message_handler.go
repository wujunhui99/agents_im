package media

import (
	"net/http"

	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// GetAvatarMessageHandler serves public avatar display via the message-api service
// context, where object storage is wired. It redirects to a short-lived presigned
// URL on the configured (external) object storage endpoint.
func GetAvatarMessageHandler(svcCtx *messagesvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetMediaDownloadURLReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		if svcCtx == nil || svcCtx.MediaLogic == nil {
			httpx.ErrorCtx(r.Context(), w, apperror.Internal("media logic is not configured"))
			return
		}

		result, err := svcCtx.MediaLogic.GetAvatarDisplayURL(r.Context(), req.MediaID)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, result.DownloadURL, http.StatusTemporaryRedirect)
	}
}
