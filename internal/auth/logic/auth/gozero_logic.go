package auth

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/apperror"
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/auth/svc"
	"github.com/wujunhui99/agents_im/internal/types"
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

func (l *RegisterLogic) Register(req *types.RegisterReq) (*types.AuthResp, error) {
	result, err := l.svcCtx.AuthLogic.Register(l.ctx, business.RegisterRequest{
		Identifier:  req.Identifier,
		Password:    req.Password,
		DisplayName: req.DisplayName,
		Name:        req.Name,
		Gender:      req.Gender,
		BirthDate:   req.BirthDate,
		Region:      req.Region,
	})
	if err != nil {
		return nil, err
	}
	return authResp(result), nil
}

type ValidateTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ValidateTokenLogic) ValidateToken(req *types.ValidateTokenReq) (*types.ValidateTokenResp, error) {
	result, err := l.svcCtx.AuthLogic.ValidateToken(l.ctx, business.ValidateTokenRequest{Token: req.Token})
	if err != nil {
		return nil, err
	}
	return &types.ValidateTokenResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ValidateTokenData{
			Valid:      result.Valid,
			UserID:     result.UserID,
			Identifier: result.Identifier,
			ExpiresAt:  result.ExpiresAt,
		},
	}, nil
}

func authResp(result business.AuthResponse) *types.AuthResp {
	return &types.AuthResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.AuthData{
			UserID:     result.UserID,
			Identifier: result.Identifier,
			Token:      result.Token,
			ExpiresAt:  result.ExpiresAt,
		},
	}
}
