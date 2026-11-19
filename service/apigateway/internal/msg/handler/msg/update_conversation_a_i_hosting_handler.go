// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package msg

import (
	"net/http"

	"github.com/wujunhui99/agents_im/service/apigateway/internal/msg/logic/msg"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/msg/svc"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/msg/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func UpdateConversationAIHostingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateConversationAIHostingReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := msg.NewUpdateConversationAIHostingLogic(r.Context(), svcCtx)
		resp, err := l.UpdateConversationAIHosting(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
