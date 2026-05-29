package friends

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/ctxuser"
	business "github.com/wujunhui99/agents_im/internal/logic"
	friendssvc "github.com/wujunhui99/agents_im/internal/servicecontext/friends"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListFriendsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *friendssvc.ServiceContext
}

func NewListFriendsLogic(ctx context.Context, svcCtx *friendssvc.ServiceContext) *ListFriendsLogic {
	return &ListFriendsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListFriendsLogic) ListFriends(req *types.ListFriendsReq) (*types.ListFriendsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.ListFriends(l.ctx, business.ListFriendsRequest{UserID: userID})
	if err != nil {
		return nil, err
	}

	friends := make([]types.Friendship, 0, len(result.Friends))
	for _, friendship := range result.Friends {
		view, err := toFriendship(l.ctx, l.svcCtx, friendship)
		if err != nil {
			return nil, err
		}
		friends = append(friends, view)
	}
	return &types.ListFriendsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data:    types.ListFriendsData{Friends: friends},
	}, nil
}
