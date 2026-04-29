package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
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
	profile, err := l.svcCtx.UserLogic.GetUserByID(l.ctx, business.GetUserByIDRequest{
		UserID: in.GetUserId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toUserResponse(profile), nil
}
