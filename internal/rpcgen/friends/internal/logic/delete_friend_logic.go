package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
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
	result, err := l.svcCtx.FriendsLogic.DeleteFriend(l.ctx, business.DeleteFriendRequest{
		UserID:   in.GetUserId(),
		FriendID: in.GetFriendId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friendspb.DeleteFriendResponse{
		Friendship: toFriendship(result.Friendship),
		Deleted:    result.Deleted,
	}, nil
}
