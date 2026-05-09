package user

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/logic"
	usersvc "github.com/wujunhui99/agents_im/internal/servicecontext/user"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetAccountByIdentifierLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *usersvc.ServiceContext
}

func NewGetAccountByIdentifierLogic(ctx context.Context, svcCtx *usersvc.ServiceContext) *GetAccountByIdentifierLogic {
	return &GetAccountByIdentifierLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetAccountByIdentifierLogic) GetAccountByIdentifier(req *types.GetUserByIdentifierReq) (*types.UserResp, error) {
	profile, err := l.svcCtx.UserLogic.GetUserByIdentifier(l.ctx, business.GetUserByIdentifierRequest{
		Identifier: req.Identifier,
	})
	if err != nil {
		return nil, err
	}
	return userRespWithAvatar(l.ctx, l.svcCtx, profile)
}
