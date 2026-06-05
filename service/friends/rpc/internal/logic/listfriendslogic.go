package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFriendsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFriendsLogic {
	return &ListFriendsLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// ListFriends 列出某用户的 accepted 好友。跨域好友资料由 api(BFF) 补全。
func (l *ListFriendsLogic) ListFriends(in *friends.ListFriendsRequest) (*friends.ListFriendsResponse, error) {
	if _, err := validateRequiredID(in.GetUserId(), "user_id"); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	rows, err := l.svcCtx.FriendshipModel.ListByAccountStatus(l.ctx, in.GetUserId(), model.FriendshipStatusAccepted)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	items := make([]*friends.Friendship, 0, len(rows))
	for _, row := range rows {
		items = append(items, toFriendship(row))
	}
	return &friends.ListFriendsResponse{Friends: items}, nil
}
