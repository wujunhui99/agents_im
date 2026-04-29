package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/internal/rpcgen/user/internal/svc"
	"github.com/wujunhui99/agents_im/proto/userpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateUserLogic {
	return &CreateUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateUserLogic) CreateUser(in *userpb.CreateUserRequest) (*userpb.UserResponse, error) {
	profile, err := l.svcCtx.UserLogic.CreateUser(l.ctx, business.CreateUserRequest{
		Identifier:  in.GetIdentifier(),
		DisplayName: in.GetDisplayName(),
		Name:        in.GetName(),
		Gender:      in.GetGender(),
		Age:         in.GetAge(),
		Region:      in.GetRegion(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toUserResponse(profile), nil
}
