package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/svc"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserAvatarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserAvatarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserAvatarLogic {
	return &UpdateUserAvatarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserAvatarLogic) UpdateUserAvatar(in *userpb.UpdateUserAvatarRequest) (*userpb.UserResponse, error) {
	if _, err := l.svcCtx.MediaLogic.ValidateAvatarMedia(l.ctx, in.GetUserId(), in.GetAvatarMediaId()); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	profile, err := l.svcCtx.UserLogic.UpdateUserAvatar(l.ctx, business.UpdateUserAvatarRequest{
		UserID:  in.GetUserId(),
		MediaID: in.GetAvatarMediaId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toUserResponse(profile), nil
}
