package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type AcceptFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAcceptFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcceptFriendLogic {
	return &AcceptFriendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AcceptFriendLogic) AcceptFriend(in *friendspb.AcceptFriendRequest) (*friendspb.AcceptFriendResponse, error) {
	result, err := l.svcCtx.FriendsLogic.AcceptFriend(l.ctx, business.AcceptFriendRequest{
		UserID:   in.GetUserId(),
		FriendID: in.GetFriendId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friendspb.AcceptFriendResponse{
		Friendship: toFriendship(result.Friendship),
		Accepted:   result.Accepted,
	}, nil
}
