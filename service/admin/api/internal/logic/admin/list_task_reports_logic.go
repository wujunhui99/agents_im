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

type ListTaskReportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTaskReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTaskReportsLogic {
	return &ListTaskReportsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTaskReportsLogic) ListTaskReports(req *types.AdminTaskReportListReq) (resp *types.AdminTaskReportListResp, err error) {
	out, err := l.svcCtx.AdminRPC.ListTaskReports(l.ctx, &adminpb.TaskReportListRequest{
		Outcome: req.Outcome,
		Limit:   int32(req.Limit),
		Offset:  int32(req.Offset),
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminTaskReportListResp{Code: codeOK, Message: messageOK, Data: types.AdminTaskReportListData{Items: adminTaskReports(out.GetItems())}}, nil
}
