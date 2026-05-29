// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package user

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMeAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMeAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMeAvatarLogic {
	return &UpdateMeAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMeAvatarLogic) UpdateMeAvatar(req *types.UpdateMeAvatarReq) (resp *types.UserResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	userResp, err := l.svcCtx.UserRPC.UpdateUserAvatar(l.ctx, &userpb.UpdateUserAvatarRequest{
		UserId:        userID,
		AvatarMediaId: req.MediaID,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return userRespFromRPC(userResp)
}
