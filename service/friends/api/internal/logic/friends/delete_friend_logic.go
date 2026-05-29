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

type DeleteFriendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFriendLogic {
	return &DeleteFriendLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DeleteFriendLogic) DeleteFriend(req *types.FriendPathReq) (resp *types.DeleteFriendResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsRPC.DeleteFriend(l.ctx, &friendspb.DeleteFriendRequest{UserId: userID, FriendId: req.UserID})
	if err != nil {
		return nil, apiError(err)
	}
	friendship, err := friendshipFromRPC(result.GetFriendship())
	if err != nil {
		return nil, err
	}
	return &types.DeleteFriendResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.DeleteFriendData{Friendship: friendship, Deleted: result.GetDeleted()}}, nil
}
