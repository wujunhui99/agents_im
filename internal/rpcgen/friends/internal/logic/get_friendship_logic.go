package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/friends/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/friendspb"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFriendshipLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetFriendshipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFriendshipLogic {
	return &GetFriendshipLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetFriendshipLogic) GetFriendship(in *friendspb.GetFriendshipRequest) (*friendspb.GetFriendshipResponse, error) {
	result, err := l.svcCtx.FriendsLogic.GetFriendship(l.ctx, business.GetFriendshipRequest{
		UserID:   in.GetUserId(),
		FriendID: in.GetFriendId(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friendspb.GetFriendshipResponse{
		Friendship: toFriendship(result.Friendship),
	}, nil
}
