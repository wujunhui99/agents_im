package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFriendshipLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFriendshipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFriendshipLogic {
	return &GetFriendshipLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetFriendshipLogic) GetFriendship(in *friendspb.GetFriendshipRequest) (*friendspb.GetFriendshipResponse, error) {
	// todo: add your logic here and delete this line

	return &friendspb.GetFriendshipResponse{}, nil
}
