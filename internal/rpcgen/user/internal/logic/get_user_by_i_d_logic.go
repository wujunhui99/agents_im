package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/user/internal/svc"
	"github.com/wujunhui99/agents_im/proto/userpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByIDLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUserByIDLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUserByIDLogic {
	return &GetUserByIDLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUserByIDLogic) GetUserByID(in *userpb.GetUserByIDRequest) (*userpb.UserResponse, error) {
	// todo: add your logic here and delete this line

	return &userpb.UserResponse{}, nil
}
