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

type GetUserLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserLogic {
	return &GetUserLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserLogic) GetUser(req *types.AdminUserReq) (resp *types.AdminUserDetailResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetUserDetail(l.ctx, &adminpb.UserDetailRequest{AccountId: req.AccountID})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminUserDetailResp{Code: codeOK, Message: messageOK, Data: types.AdminUserDetailData{User: adminUser(out.GetUser())}}, nil
}
