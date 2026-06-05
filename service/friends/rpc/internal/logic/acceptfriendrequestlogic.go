package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
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

// AcceptFriendRequest 接受好友请求：user_id 接受来自 friend_id 的待定请求，双向置 accepted。
func (l *AcceptFriendRequestLogic) AcceptFriendRequest(in *friends.FriendRequestDecisionRequest) (*friends.FriendRequestDecisionResponse, error) {
	userID, requesterID, err := validateFriendshipPair(in.GetUserId(), in.GetFriendId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	row, err := decideFriendRequest(l.ctx, l.svcCtx.FriendshipModel, userID, requesterID, model.FriendshipStatusAccepted)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.FriendRequestDecisionResponse{Friendship: toFriendship(row), Updated: true}, nil
}
