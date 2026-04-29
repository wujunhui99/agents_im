package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
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
	result, err := l.svcCtx.FriendsLogic.ListFriends(l.ctx, business.ListFriendsRequest{
		UserID: in.GetUserId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	friends := make([]*friendspb.Friendship, 0, len(result.Friends))
	for _, friend := range result.Friends {
		friends = append(friends, toFriendship(friend))
	}
	return &friendspb.ListFriendsResponse{Friends: friends}, nil
}
