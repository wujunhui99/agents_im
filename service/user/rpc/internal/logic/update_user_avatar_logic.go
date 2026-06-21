package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/service/user/rpc/internal/model"
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
	// 校验头像 media 存在/归属/类型（media 域，经 media-rpc，#533）。校验器已返回 gRPC status，
	// 直接透传，勿再经 rpcerror.ToStatus（会把 InvalidArgument/NotFound 等折成 Internal）。
	if err := l.svcCtx.AvatarValidator.ValidateAvatarMedia(l.ctx, in.GetUserId(), in.GetAvatarMediaId()); err != nil {
		return nil, err
	}

	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("user_id is required"))
	}
	mediaID := strings.TrimSpace(in.GetAvatarMediaId())
	if mediaID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("media_id is required"))
	}
	// wire 是十进制串、DB 是 bigint(#550):转成 int64 落库;URL 仍用十进制串渲染。
	avatarMediaID, err := model.ParseAvatarMediaID(mediaID)
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("avatar_media_id must be a decimal media id"))
	}

	if err := l.svcCtx.Profiles.UpdateAvatar(l.ctx, userID, avatarMediaID, DurableAvatarURL(mediaID)); err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	ap, err := l.svcCtx.Accounts.FindAccountProfileByID(l.ctx, userID)
	if err != nil {
		return nil, rpcerror.ToStatus(mapReadError(err))
	}
	return toUserResponse(ap), nil
}
