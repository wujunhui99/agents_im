// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package friends

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/friends/api/internal/types"
	friendspb "github.com/wujunhui99/agents_im/service/friends/rpc/friends"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFriendshipLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFriendshipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFriendshipLogic {
	return &GetFriendshipLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *GetFriendshipLogic) GetFriendship(req *types.FriendPathReq) (resp *types.FriendshipResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsRPC.GetFriendship(l.ctx, &friendspb.GetFriendshipRequest{UserId: userID, FriendId: req.UserID})
	if err != nil {
		return nil, apiError(err)
	}
	friendship, err := friendshipFromRPC(result.GetFriendship())
	if err != nil {
		return nil, err
	}
	return &types.FriendshipResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.FriendshipData{Friendship: friendship}}, nil
}
