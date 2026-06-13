package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	business "github.com/wujunhui99/agents_im/service/auth/core/logic"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

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

func (l *LoginLogic) Login(in *auth.LoginRequest) (*auth.AuthResponse, error) {
	result, err := l.svcCtx.AuthLogic.Login(l.ctx, business.LoginRequest{
		Identifier: in.GetIdentifier(),
		Password:   in.GetPassword(),
		Device:     in.GetDevice(),
		LoginIP:    in.GetLoginIp(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toAuthResponse(result), nil
}
