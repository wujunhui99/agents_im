package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendRequestsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFriendRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendRequestsLogic {
	return &ListFriendRequestsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ListFriendRequests 列出待定好友请求：incoming（他人发给我，按发起方排序）、outgoing（我发给他人）。
// 待显示的对端资料由 api(BFF) 补全：incoming 端是 user_id（发起方），outgoing 端是 friend_id。
func (l *ListFriendRequestsLogic) ListFriendRequests(in *friends.ListFriendRequestsRequest) (*friends.ListFriendRequestsResponse, error) {
	if _, err := validateRequiredID(in.GetUserId(), "user_id"); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	incomingRows, err := l.svcCtx.FriendshipModel.ListByFriendStatus(l.ctx, in.GetUserId(), model.FriendshipStatusPending)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	outgoingRows, err := l.svcCtx.FriendshipModel.ListByAccountStatus(l.ctx, in.GetUserId(), model.FriendshipStatusPending)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	incoming := make([]*friends.Friendship, 0, len(incomingRows))
	for _, row := range incomingRows {
		incoming = append(incoming, toFriendship(row))
	}
	outgoing := make([]*friends.Friendship, 0, len(outgoingRows))
	for _, row := range outgoingRows {
		outgoing = append(outgoing, toFriendship(row))
	}
	return &friends.ListFriendRequestsResponse{Incoming: incoming, Outgoing: outgoing}, nil
}
