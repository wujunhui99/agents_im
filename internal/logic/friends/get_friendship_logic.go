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

type GetFriendshipLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *friendssvc.ServiceContext
}

func NewGetFriendshipLogic(ctx context.Context, svcCtx *friendssvc.ServiceContext) *GetFriendshipLogic {
	return &GetFriendshipLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFriendshipLogic) GetFriendship(req *types.FriendPathReq) (*types.FriendshipResp, error) {
	userID, err := ctxuser.UserID(l.ctx)
	if err != nil {
		return nil, err
	}

	result, err := l.svcCtx.FriendsLogic.GetFriendship(l.ctx, business.GetFriendshipRequest{
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
	return &types.FriendshipResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.FriendshipData{
			Friendship: friendship,
		},
	}, nil
}
