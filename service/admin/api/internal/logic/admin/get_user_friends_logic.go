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

type GetUserFriendsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUserFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserFriendsLogic {
	return &GetUserFriendsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserFriendsLogic) GetUserFriends(req *types.AdminUserReq) (resp *types.AdminUserFriendsResp, err error) {
	out, err := l.svcCtx.AdminRPC.GetUserFriends(l.ctx, &adminpb.UserFriendsRequest{AccountId: req.AccountID})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	return &types.AdminUserFriendsResp{Code: codeOK, Message: messageOK, Data: types.AdminUserFriendsData{Friends: adminFriends(out.GetFriends())}}, nil
}
