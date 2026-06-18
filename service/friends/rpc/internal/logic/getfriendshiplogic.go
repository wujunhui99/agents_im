package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
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

// GetFriendship 查询 user_id 视角下与 friend_id 的关系：命中正向行直接返回；
// 仅有反向 pending（对方发来未处理的请求）则合成一条 pending；都没有则返回 none。
func (l *GetFriendshipLogic) GetFriendship(in *friends.GetFriendshipRequest) (*friends.GetFriendshipResponse, error) {
	userID, friendID, err := validateFriendshipPair(in.GetUserId(), in.GetFriendId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	row, err := l.svcCtx.FriendshipModel.FindOneByAccountIdFriendAccountId(l.ctx, userID, friendID)
	if err == nil {
		return &friends.GetFriendshipResponse{Friendship: toFriendship(row)}, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return nil, rpcerror.ToStatus(err)
	}

	reverse, reverseErr := l.svcCtx.FriendshipModel.FindOneByAccountIdFriendAccountId(l.ctx, friendID, userID)
	if reverseErr == nil && reverse.Status == model.FriendshipStatusPending {
		view := friendshipView(userID, friendID, model.FriendshipStatusPending, reverse.CreatedAt, reverse.UpdatedAt)
		return &friends.GetFriendshipResponse{Friendship: view}, nil
	}
	if reverseErr != nil && !errors.Is(reverseErr, model.ErrNotFound) {
		return nil, rpcerror.ToStatus(reverseErr)
	}

	return &friends.GetFriendshipResponse{Friendship: noneFriendship(userID, friendID)}, nil
}
