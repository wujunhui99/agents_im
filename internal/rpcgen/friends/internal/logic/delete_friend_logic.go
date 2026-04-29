package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFriendLogic {
	return &DeleteFriendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteFriendLogic) DeleteFriend(in *friendspb.DeleteFriendRequest) (*friendspb.DeleteFriendResponse, error) {
	// todo: add your logic here and delete this line

	return &friendspb.DeleteFriendResponse{}, nil
}
