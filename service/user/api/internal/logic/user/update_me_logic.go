// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package user

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	"github.com/wujunhui99/agents_im/proto/userpb"
	"github.com/wujunhui99/agents_im/service/user/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateMeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMeLogic {
	return &UpdateMeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateMeLogic) UpdateMe(req *types.UpdateMeReq) (resp *types.UserResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.UserID) != "" || strings.TrimSpace(req.Identifier) != "" {
		return nil, apperror.InvalidArgument("immutable profile fields cannot be updated")
	}
	userResp, err := l.svcCtx.UserRPC.UpdateUserProfile(l.ctx, &userpb.UpdateUserProfileRequest{
		UserId:      userID,
		DisplayName: optionalString(req.DisplayName),
		Name:        optionalString(req.Name),
		Gender:      optionalString(req.Gender),
		BirthDate:   optionalString(req.BirthDate),
		Region:      optionalString(req.Region),
	})
	if err != nil {
		return nil, apiError(err)
	}
	return userRespFromRPC(userResp)
}
