// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLLMTraceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetLLMTraceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLLMTraceLogic {
	return &GetLLMTraceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLLMTraceLogic) GetLLMTrace(req *types.AdminLLMTraceReq) (resp *types.AdminLLMTraceDetailResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetLLMTraceDetail(l.ctx, &adminpb.LLMTraceDetailRequest{TraceId: req.TraceID})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminLLMTraceDetailResp{Code: codeOK, Message: messageOK, Data: types.AdminLLMTraceDetailData{
		Trace:       adminTrace(out.GetTrace()),
		ToolCalls:   adminToolCalls(out.GetToolCalls()),
		FileReads:   adminFileReads(out.GetFileReads()),
		PythonExecs: adminPythonExecs(out.GetPythonExecs()),
	}}, nil
}
