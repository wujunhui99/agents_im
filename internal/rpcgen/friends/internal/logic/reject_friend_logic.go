package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RejectFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRejectFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejectFriendLogic {
	return &RejectFriendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RejectFriendLogic) RejectFriend(in *friendspb.RejectFriendRequest) (*friendspb.RejectFriendResponse, error) {
	result, err := l.svcCtx.FriendsLogic.RejectFriend(l.ctx, business.RejectFriendRequest{
		UserID:   in.GetUserId(),
		FriendID: in.GetFriendId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friendspb.RejectFriendResponse{
		Friendship: toFriendship(result.Friendship),
		Rejected:   result.Rejected,
	}, nil
}
