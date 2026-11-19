// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/admin/svc"
	"github.com/wujunhui99/agents_im/service/apigateway/internal/admin/types"
	adminpb "github.com/wujunhui99/agents_im_protocol/admin/v1"

	"github.com/zeromicro/go-zero/core/logx"
)

type DashboardLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDashboardLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DashboardLogic {
	return &DashboardLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DashboardLogic) Dashboard(req *types.AdminDashboardReq) (resp *types.AdminDashboardResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetDashboard(l.ctx, &adminpb.DashboardRequest{Limit: int32(req.Limit)})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminDashboardResp{Code: codeOK, Message: messageOK, Data: adminDashboardData(out)}, nil
}
