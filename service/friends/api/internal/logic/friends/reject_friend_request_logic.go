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

type RejectFriendRequestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRejectFriendRequestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RejectFriendRequestLogic {
	return &RejectFriendRequestLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RejectFriendRequestLogic) RejectFriendRequest(req *types.FriendPathReq) (resp *types.FriendRequestDecisionResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsRPC.RejectFriendRequest(l.ctx, &friendspb.FriendRequestDecisionRequest{UserId: userID, FriendId: req.UserID})
	if err != nil {
		return nil, apiError(err)
	}
	friendship, err := friendshipFromRPC(result.GetFriendship())
	if err != nil {
		return nil, err
	}
	return &types.FriendRequestDecisionResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.FriendRequestDecisionData{Friendship: friendship, Updated: result.GetUpdated()}}, nil
}
