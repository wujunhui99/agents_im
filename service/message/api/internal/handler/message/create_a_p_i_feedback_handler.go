// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package message

import (
	"net/http"

	"github.com/wujunhui99/agents_im/service/message/api/internal/logic/message"
	"github.com/wujunhui99/agents_im/service/message/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/message/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateAPIFeedbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateFeedbackReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := message.NewCreateAPIFeedbackLogic(r.Context(), svcCtx)
		resp, err := l.CreateAPIFeedback(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
