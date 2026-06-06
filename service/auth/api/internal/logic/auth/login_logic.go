// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/wujunhui99/agents_im/service/auth/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LoginLogic) Login(req *types.LoginReq) (resp *types.AuthResp, err error) {
	result, err := l.svcCtx.AuthRPC.Login(l.ctx, &authpb.LoginRequest{
		Identifier: req.Identifier,
		Password:   req.Password,
		Device:     req.Device,
		LoginIp:    clientIP(l.ctx),
	})
	if err != nil {
		return nil, apiError(err)
	}
	return authResp(result)
}
