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

type ListFriendRequestsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *friendssvc.ServiceContext
}

func NewListFriendRequestsLogic(ctx context.Context, svcCtx *friendssvc.ServiceContext) *ListFriendRequestsLogic {
	return &ListFriendRequestsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListFriendRequestsLogic) ListFriendRequests(_ *types.ListFriendRequestsReq) (*types.ListFriendRequestsResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.ListFriendRequests(l.ctx, business.ListFriendRequestsRequest{UserID: userID})
	if err != nil {
		return nil, err
	}

	incoming := make([]types.Friendship, 0, len(result.Incoming))
	for _, friendship := range result.Incoming {
		view, err := toFriendship(l.ctx, l.svcCtx, friendship)
		if err != nil {
			return nil, err
		}
		incoming = append(incoming, view)
	}
	outgoing := make([]types.Friendship, 0, len(result.Outgoing))
	for _, friendship := range result.Outgoing {
		view, err := toFriendship(l.ctx, l.svcCtx, friendship)
		if err != nil {
			return nil, err
		}
		outgoing = append(outgoing, view)
	}

	return &types.ListFriendRequestsResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ListFriendRequestsData{
			Incoming: incoming,
			Outgoing: outgoing,
		},
	}, nil
}
