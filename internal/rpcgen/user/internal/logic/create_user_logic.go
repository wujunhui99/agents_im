package logic

import (
	"context"

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
	// todo: add your logic here and delete this line

	return &userpb.UserResponse{}, nil
}
