package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DeleteFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteFriendLogic {
	return &DeleteFriendLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// DeleteFriend 删除好友：双向把 accepted 关系置为 deleted；无 accepted 关系返回 NotFound。
func (l *DeleteFriendLogic) DeleteFriend(in *friends.DeleteFriendRequest) (*friends.DeleteFriendResponse, error) {
	userID, friendID, err := validateFriendshipPair(in.GetUserId(), in.GetFriendId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	var row *model.Friendships
	err = l.svcCtx.FriendshipModel.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		m := l.svcCtx.FriendshipModel.WithSession(session)
		deleted, err := m.MarkAcceptedDeleted(ctx, userID, friendID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return apperror.NotFound("friendship not found")
			}
			return err
		}
		if err := m.MarkAcceptedDeletedSilent(ctx, friendID, userID); err != nil {
			return err
		}
		row = deleted
		return nil
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	return &friends.DeleteFriendResponse{Friendship: toFriendship(row), Deleted: true}, nil
}
