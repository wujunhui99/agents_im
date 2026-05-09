// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package agent

import (
	"net/http"

	"github.com/wujunhui99/agents_im/internal/logic/agent"
	agentsvc "github.com/wujunhui99/agents_im/internal/servicecontext/agent"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CreateAgentHandler(svcCtx *agentsvc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateAgentReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := agent.NewCreateAgentLogic(r.Context(), svcCtx)
		resp, err := l.CreateAgent(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
