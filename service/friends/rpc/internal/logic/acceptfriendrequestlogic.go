package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/service/friends/core"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AcceptFriendRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAcceptFriendRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AcceptFriendRequestLogic {
	return &AcceptFriendRequestLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *AcceptFriendRequestLogic) AcceptFriendRequest(in *friends.FriendRequestDecisionRequest) (*friends.FriendRequestDecisionResponse, error) {
	result, err := l.svcCtx.FriendsLogic.AcceptFriendRequest(l.ctx, business.FriendRequestDecisionRequest{UserID: in.GetUserId(), FriendID: in.GetFriendId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.FriendRequestDecisionResponse{Friendship: toFriendship(result.Friendship), Updated: result.Updated}, nil
}
