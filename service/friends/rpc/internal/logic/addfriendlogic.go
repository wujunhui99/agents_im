package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddFriendLogic {
	return &AddFriendLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AddFriendLogic) AddFriend(in *friends.AddFriendRequest) (*friends.AddFriendResponse, error) {
	result, err := l.svcCtx.FriendsLogic.AddFriend(l.ctx, business.AddFriendRequest{UserID: in.GetUserId(), FriendID: in.GetFriendId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.AddFriendResponse{Friendship: toFriendship(result.Friendship), Created: result.Created}, nil
}
