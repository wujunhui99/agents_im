// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package media

import (
	"net/http"

	medialogic "github.com/wujunhui99/agents_im/internal/logic/media"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateUploadIntentHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateMediaUploadReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := medialogic.NewCreateUploadIntentLogic(r.Context(), svcCtx)
		resp, err := l.CreateUploadIntent(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
