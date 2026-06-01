package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/service/friends/core"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFriendshipLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFriendshipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFriendshipLogic {
	return &GetFriendshipLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

func (l *GetFriendshipLogic) GetFriendship(in *friends.GetFriendshipRequest) (*friends.GetFriendshipResponse, error) {
	result, err := l.svcCtx.FriendsLogic.GetFriendship(l.ctx, business.GetFriendshipRequest{UserID: in.GetUserId(), FriendID: in.GetFriendId()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.GetFriendshipResponse{Friendship: toFriendship(result.Friendship)}, nil
}
