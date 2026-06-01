package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFriendLogic {
	return &DeleteFriendLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *DeleteFriendLogic) DeleteFriend(in *friends.DeleteFriendRequest) (*friends.DeleteFriendResponse, error) {
	result, err := l.svcCtx.FriendsLogic.DeleteFriend(l.ctx, business.DeleteFriendRequest{UserID: in.GetUserId(), FriendID: in.GetFriendId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.DeleteFriendResponse{Friendship: toFriendship(result.Friendship), Deleted: result.Deleted}, nil
}
