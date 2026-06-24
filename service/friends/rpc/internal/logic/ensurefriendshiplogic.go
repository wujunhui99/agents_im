package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	friends "github.com/wujunhui99/agents_im/service/friends/rpc/friends"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/friends/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type EnsureFriendshipLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnsureFriendshipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnsureFriendshipLogic {
	return &EnsureFriendshipLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

// EnsureFriendship 幂等建立 user_id <-> friend_id 的双向 accepted 关系。两个方向在同一事务内
// upsert（保留 created_at），无待批流程。取代旧 internal/repository.EnsureAcceptedFriendship；
// 账号存在性由调用方负责（friends-rpc 是叶子，不读 accounts）。
func (l *EnsureFriendshipLogic) EnsureFriendship(in *friends.EnsureFriendshipRequest) (*friends.EnsureFriendshipResponse, error) {
	userID, friendID, err := validateFriendshipPair(in.GetUserId(), in.GetFriendId())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	fm := l.svcCtx.FriendshipModel
	var forward *model.Friendships
	err = fm.Transact(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		m := fm.WithSession(session)
		row, err := m.EnsureAccepted(ctx, userID, friendID)
		if err != nil {
			return err
		}
		if _, err := m.EnsureAccepted(ctx, friendID, userID); err != nil {
			return err
		}
		forward = row
		return nil
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &friends.EnsureFriendshipResponse{Friendship: toFriendship(forward)}, nil
}
