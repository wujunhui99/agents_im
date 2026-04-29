package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/authpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *authpb.LoginRequest) (*authpb.AuthResponse, error) {
	result, err := l.svcCtx.AuthLogic.Login(l.ctx, business.LoginRequest{
		Identifier: in.GetIdentifier(),
		Password:   in.GetPassword(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toAuthResponse(result), nil
}
