package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RejectFriendRequestLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRejectFriendRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejectFriendRequestLogic {
	return &RejectFriendRequestLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// RejectFriendRequest 拒绝好友请求：user_id 拒绝来自 friend_id 的待定请求，双向置 rejected。
func (l *RejectFriendRequestLogic) RejectFriendRequest(in *friends.FriendRequestDecisionRequest) (*friends.FriendRequestDecisionResponse, error) {
	userID, requesterID, err := validateFriendshipPair(in.GetUserId(), in.GetFriendId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	row, err := decideFriendRequest(l.ctx, l.svcCtx.FriendshipModel, userID, requesterID, model.FriendshipStatusRejected)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.FriendRequestDecisionResponse{Friendship: toFriendship(row), Updated: true}, nil
}
