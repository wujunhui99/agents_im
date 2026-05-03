// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package friends

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/logic/friends"
	"github.com/wujunhui99/agents_im/internal/svc"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AcceptFriendRequestHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.FriendPathReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := friends.NewAcceptFriendRequestLogic(r.Context(), svcCtx)
		resp, err := l.AcceptFriendRequest(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
