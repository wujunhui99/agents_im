package friends

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	friendssvc "github.com/wujunhui99/agents_im/internal/servicecontext/friends"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type AcceptFriendRequestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *friendssvc.ServiceContext
}

func NewAcceptFriendRequestLogic(ctx context.Context, svcCtx *friendssvc.ServiceContext) *AcceptFriendRequestLogic {
	return &AcceptFriendRequestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AcceptFriendRequestLogic) AcceptFriendRequest(req *types.FriendPathReq) (*types.FriendRequestDecisionResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}
	result, err := l.svcCtx.FriendsLogic.AcceptFriendRequest(l.ctx, business.FriendRequestDecisionRequest{
		UserID:   userID,
		FriendID: req.UserID,
	})
	if err != nil {
		return nil, err
	}
	friendship, err := toFriendship(l.ctx, l.svcCtx, result.Friendship)
	if err != nil {
		return nil, err
	}
	return &types.FriendRequestDecisionResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FriendRequestDecisionData{
			Friendship: friendship,
			Updated:    result.Updated,
		},
	}, nil
}
