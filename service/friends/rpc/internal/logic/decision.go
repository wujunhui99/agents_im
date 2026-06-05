package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// decideFriendRequest 处理一条待定好友请求（accept/reject 共用）：
// 入参 userID 为决定者、requesterID 为发起者，待定行为 requester -> user。
// 两个方向同步更新为 newStatus，返回 user -> requester 行；无待定请求返回 NotFound。
func decideFriendRequest(ctx context.Context, fm model.FriendshipsModel, userID, requesterID string, newStatus int64) (*model.Friendships, error) {
	var row *model.Friendships
	err := fm.Transact(ctx, func(ctx context.Context, session sqlx.Session) error {
		m := fm.WithSession(session)

		incoming, err := m.FindPairForUpdate(ctx, requesterID, userID)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				return apperror.NotFound("friend request not found")
			}
			return err
		}
		if incoming.Status != model.FriendshipStatusPending {
			return apperror.NotFound("friend request not found")
		}
		if _, err := m.UpsertStatus(ctx, requesterID, userID, newStatus); err != nil {
			return err
		}
		updated, err := m.UpsertStatus(ctx, userID, requesterID, newStatus)
		if err != nil {
			return err
		}
		row = updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}
