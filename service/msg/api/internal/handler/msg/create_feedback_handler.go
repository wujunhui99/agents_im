// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"net/http"

	"github.com/wujunhui99/agents_im/service/msg/api/internal/logic/msg"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateFeedbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateFeedbackReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := msg.NewCreateFeedbackLogic(r.Context(), svcCtx)
		resp, err := l.CreateFeedback(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
