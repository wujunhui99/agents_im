// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	adminpb "github.com/wujunhui99/agents_im/service/admin/rpc/admin"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListLLMTracesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLLMTracesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLLMTracesLogic {
	return &ListLLMTracesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListLLMTracesLogic) ListLLMTraces(req *types.AdminLLMTraceListReq) (resp *types.AdminLLMTraceListResp, err error) {
	out, err := l.svcCtx.AdminRPC.ListLLMTraces(l.ctx, &adminpb.LLMTraceListRequest{
		Status: req.Status,
		Limit:  int32(req.Limit),
		Offset: int32(req.Offset),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminLLMTraceListResp{Code: codeOK, Message: messageOK, Data: types.AdminLLMTraceListData{Traces: adminTraces(out.GetTraces())}}, nil
}
