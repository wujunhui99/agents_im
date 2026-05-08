package user

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMeAvatarLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewUpdateMeAvatarLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *UpdateMeAvatarLogic {
	return &UpdateMeAvatarLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMeAvatarLogic) UpdateMeAvatar(req *types.UpdateMeAvatarReq) (*types.UserResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if l.svcCtx.MediaLogic == nil {
		return nil, apperror.Internal("media logic is not configured")
	}
	if _, err := l.svcCtx.MediaLogic.ValidateAvatarMedia(l.ctx, userID, req.MediaID); err != nil {
		return nil, err
	}

	profile, err := l.svcCtx.UserLogic.UpdateUserAvatar(l.ctx, business.UpdateUserAvatarRequest{
		UserID:  userID,
		MediaID: req.MediaID,
	})
	if err != nil {
		return nil, err
	}
	return userRespWithAvatar(l.ctx, l.svcCtx, profile)
}
