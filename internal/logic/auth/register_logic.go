package auth

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *authsvc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *authsvc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterLogic) Register(req *types.RegisterReq) (*types.AuthResp, error) {
	result, err := l.svcCtx.AuthLogic.Register(l.ctx, business.RegisterRequest{
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
		return nil, err
	}
	return authResp(result), nil
}
