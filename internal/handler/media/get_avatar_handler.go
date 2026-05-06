package media

import (
	"net/http"

	medialogic "github.com/wujunhui99/agents_im/internal/logic/media"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetAvatarHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetMediaDownloadURLReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := medialogic.NewGetAvatarLogic(r.Context(), svcCtx)
		result, err := l.GetAvatar(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, result.DownloadURL, http.StatusTemporaryRedirect)
	}
}
