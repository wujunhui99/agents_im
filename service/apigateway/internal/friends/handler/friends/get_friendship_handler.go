// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package friends

import (
	"net/http"

	"github.com/wujunhui99/agents_im/service/apigateway/internal/friends/logic/friends"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/friends/svc"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/friends/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetFriendshipHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FriendPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := friends.NewGetFriendshipLogic(r.Context(), svcCtx)
		resp, err := l.GetFriendship(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
