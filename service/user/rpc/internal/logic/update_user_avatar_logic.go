package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
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
	// keystone：校验头像 media 存在且类型合法（media 域读，仍走 internal）。
	if err := l.svcCtx.AvatarValidator.ValidateAvatarMedia(l.ctx, in.GetUserId(), in.GetAvatarMediaId()); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("user_id is required"))
	}
	mediaID := strings.TrimSpace(in.GetAvatarMediaId())
	if mediaID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("media_id is required"))
	}

	if err := l.svcCtx.Profiles.UpdateAvatar(l.ctx, userID, mediaID, DurableAvatarURL(mediaID)); err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	ap, err := l.svcCtx.Accounts.FindAccountProfileByID(l.ctx, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	return toUserResponse(ap), nil
}
