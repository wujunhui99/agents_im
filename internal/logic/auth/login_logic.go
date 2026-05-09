package auth

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *authsvc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *authsvc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (*types.AuthResp, error) {
	result, err := l.svcCtx.AuthLogic.Login(l.ctx, business.LoginRequest{
		Identifier: req.Identifier,
		Password:   req.Password,
	})
	if err != nil {
		return nil, err
	}
	return authResp(result), nil
}
