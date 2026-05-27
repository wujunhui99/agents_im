// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package groups

import (
	"net/http"

	"github.com/wujunhui99/agents_im/service/groups/api/internal/logic/groups"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/groups/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetGroupHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetGroupReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := groups.NewGetGroupLogic(r.Context(), svcCtx)
		resp, err := l.GetGroup(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
