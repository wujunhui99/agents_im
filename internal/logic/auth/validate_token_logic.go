package auth

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
	"github.com/wujunhui99/agents_im/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *authsvc.ServiceContext
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *authsvc.ServiceContext) *ValidateTokenLogic {
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
