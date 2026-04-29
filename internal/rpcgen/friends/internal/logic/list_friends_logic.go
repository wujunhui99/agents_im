package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendsLogic {
	return &ListFriendsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListFriendsLogic) ListFriends(in *friendspb.ListFriendsRequest) (*friendspb.ListFriendsResponse, error) {
	// todo: add your logic here and delete this line

	return &friendspb.ListFriendsResponse{}, nil
}
