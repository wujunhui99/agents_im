package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type AddFriendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddFriendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddFriendLogic {
	return &AddFriendLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// AddFriend 发起好友请求：双方互发即互相成为好友（pending -> accepted），否则落 pending。
func (l *AddFriendLogic) AddFriend(in *friends.AddFriendRequest) (*friends.AddFriendResponse, error) {
	userID, friendID, err := validateFriendshipPair(in.GetUserId(), in.GetFriendId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	var row *model.Friendships
	created := true
	err = l.svcCtx.FriendshipModel.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		m := l.svcCtx.FriendshipModel.WithSession(session)

		existing, err := m.FindPairForUpdate(ctx, userID, friendID)
		if err == nil && (existing.Status == model.FriendshipStatusAccepted || existing.Status == model.FriendshipStatusPending) {
			row = existing
			created = false
			return nil
		}
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			return err
		}

		reverse, reverseErr := m.FindPairForUpdate(ctx, friendID, userID)
		if reverseErr != nil && !errors.Is(reverseErr, model.ErrNotFound) {
			return reverseErr
		}
		if reverseErr == nil && reverse.Status == model.FriendshipStatusPending {
			accepted, err := m.UpsertStatus(ctx, userID, friendID, model.FriendshipStatusAccepted)
			if err != nil {
				return err
			}
			if _, err := m.UpsertStatus(ctx, friendID, userID, model.FriendshipStatusAccepted); err != nil {
				return err
			}
			row = accepted
			return nil
		}

		pending, err := m.UpsertStatus(ctx, userID, friendID, model.FriendshipStatusPending)
		if err != nil {
			return err
		}
		row = pending
		return nil
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	return &friends.AddFriendResponse{Friendship: toFriendship(row), Created: created}, nil
}
