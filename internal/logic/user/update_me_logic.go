package user

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewUpdateMeLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *UpdateMeLogic {
	return &UpdateMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMeLogic) UpdateMe(req *types.UpdateMeReq) (*types.UserResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.UserID) != "" || strings.TrimSpace(req.Identifier) != "" {
		return nil, apperror.InvalidArgument("immutable profile fields cannot be updated")
	}

	profile, err := l.svcCtx.UserLogic.UpdateUserProfile(l.ctx, business.UpdateUserProfileRequest{
		UserID:      userID,
		DisplayName: optionalString(req.DisplayName),
		Name:        optionalString(req.Name),
		Gender:      optionalString(req.Gender),
		BirthDate:   optionalString(req.BirthDate),
		Region:      optionalString(req.Region),
	})
	if err != nil {
		return nil, err
	}
	return userRespWithAvatar(l.ctx, l.svcCtx, profile)
}
