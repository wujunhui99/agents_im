package user

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetUserByIdentifierLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewGetUserByIdentifierLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *GetUserByIdentifierLogic {
	return &GetUserByIdentifierLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUserByIdentifierLogic) GetUserByIdentifier(req *types.GetUserByIdentifierReq) (*types.UserResp, error) {
	profile, err := l.svcCtx.UserLogic.GetUserByIdentifier(l.ctx, business.GetUserByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, err
	}
	return userRespWithAvatar(l.ctx, l.svcCtx, profile)
}
