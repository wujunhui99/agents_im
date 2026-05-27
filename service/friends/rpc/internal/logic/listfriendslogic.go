package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendsLogic {
	return &ListFriendsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *ListFriendsLogic) ListFriends(in *friends.ListFriendsRequest) (*friends.ListFriendsResponse, error) {
	result, err := l.svcCtx.FriendsLogic.ListFriends(l.ctx, business.ListFriendsRequest{UserID: in.GetUserId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	items := make([]*friends.Friendship, 0, len(result.Friends))
	for _, item := range result.Friends {
		items = append(items, toFriendship(item))
	}
	return &friends.ListFriendsResponse{Friends: items}, nil
}
