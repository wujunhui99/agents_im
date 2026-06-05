// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package friends

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/types"
	friendspb "github.com/wujunhui99/agents_im/service/friends/rpc/friends"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendRequestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFriendRequestsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendRequestsLogic {
	return &ListFriendRequestsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListFriendRequestsLogic) ListFriendRequests(req *types.ListFriendsReq) (resp *types.ListFriendRequestsResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsRPC.ListFriendRequests(l.ctx, &friendspb.ListFriendRequestsRequest{UserId: userID})
	if err != nil {
		return nil, apiError(err)
	}
	incoming, err := friendshipsFromRPC(result.GetIncoming())
	if err != nil {
		return nil, err
	}
	outgoing, err := friendshipsFromRPC(result.GetOutgoing())
	if err != nil {
		return nil, err
	}
	// incoming 展示发起方(user_id)资料，outgoing 展示对方(friend_id)资料。
	if err := hydrateFriendships(l.ctx, l.svcCtx, incoming, peerIsRequester); err != nil {
		return nil, err
	}
	if err := hydrateFriendships(l.ctx, l.svcCtx, outgoing, peerIsFriend); err != nil {
		return nil, err
	}
	return &types.ListFriendRequestsResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.ListFriendRequestsData{Incoming: incoming, Outgoing: outgoing}}, nil
}
