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

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterReq) (resp *types.AuthResp, err error) {
	result, err := l.svcCtx.AuthRPC.Register(l.ctx, &authpb.RegisterRequest{
		Identifier:            req.Identifier,
		Email:                 req.Email,
		EmailVerificationCode: req.EmailVerificationCode,
		Password:              req.Password,
		DisplayName:           req.DisplayName,
		Name:                  req.Name,
		Gender:                req.Gender,
		BirthDate:             req.BirthDate,
		Region:                req.Region,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return authResp(result)
}
