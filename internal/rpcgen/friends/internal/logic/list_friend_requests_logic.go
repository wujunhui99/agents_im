package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendRequestsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFriendRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendRequestsLogic {
	return &ListFriendRequestsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListFriendRequestsLogic) ListFriendRequests(in *friendspb.ListFriendRequestsRequest) (*friendspb.ListFriendRequestsResponse, error) {
	result, err := l.svcCtx.FriendsLogic.ListFriendRequests(l.ctx, business.ListFriendRequestsRequest{
		UserID: in.GetUserId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	incoming := make([]*friendspb.Friendship, 0, len(result.Incoming))
	for _, friendship := range result.Incoming {
		incoming = append(incoming, toFriendship(friendship))
	}
	outgoing := make([]*friendspb.Friendship, 0, len(result.Outgoing))
	for _, friendship := range result.Outgoing {
		outgoing = append(outgoing, toFriendship(friendship))
	}
	return &friendspb.ListFriendRequestsResponse{
		Incoming: incoming,
		Outgoing: outgoing,
	}, nil
}
