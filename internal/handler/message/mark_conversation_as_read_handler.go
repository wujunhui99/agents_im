// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package message

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/logic/message"
	messagesvc "github.com/wujunhui99/agents_im/internal/servicecontext/message"
	"github.com/wujunhui99/agents_im/common/share/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func MarkConversationAsReadHandler(svcCtx *messagesvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MarkConversationAsReadReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := message.NewMarkConversationAsReadLogic(r.Context(), svcCtx)
		resp, err := l.MarkConversationAsRead(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
