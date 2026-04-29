package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/internal/rpcgen/user/internal/svc"
	"github.com/wujunhui99/agents_im/proto/userpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateUserProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateUserProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateUserProfileLogic {
	return &UpdateUserProfileLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateUserProfileLogic) UpdateUserProfile(in *userpb.UpdateUserProfileRequest) (*userpb.UserResponse, error) {
	profile, err := l.svcCtx.UserLogic.UpdateUserProfile(l.ctx, business.UpdateUserProfileRequest{
		UserID:      in.GetUserId(),
		DisplayName: in.DisplayName,
		Name:        in.Name,
		Gender:      in.Gender,
		Age:         in.Age,
		Region:      in.Region,
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toUserResponse(profile), nil
}
