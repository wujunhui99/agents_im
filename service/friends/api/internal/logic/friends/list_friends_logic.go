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

type ListFriendsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendsLogic {
	return &ListFriendsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ListFriendsLogic) ListFriends(req *types.ListFriendsReq) (resp *types.ListFriendsResp, err error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsRPC.ListFriends(l.ctx, &friendspb.ListFriendsRequest{UserId: userID})
	if err != nil {
		return nil, apiError(err)
	}
	friends, err := friendshipsFromRPC(result.GetFriends())
	if err != nil {
		return nil, err
	}
	return &types.ListFriendsResp{Code: string(apperror.CodeOK), Message: "ok", Data: types.ListFriendsData{Friends: friends}}, nil
}
