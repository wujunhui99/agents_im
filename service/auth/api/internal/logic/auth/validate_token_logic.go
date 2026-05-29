// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"

	"github.com/zeromicro/go-zero/core/logx"
)

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

func (l *ValidateTokenLogic) ValidateToken(req *types.ValidateTokenReq) (resp *types.ValidateTokenResp, err error) {
	result, err := l.svcCtx.AuthRPC.ValidateToken(l.ctx, &authpb.ValidateTokenRequest{Token: req.Token})
	if err != nil {
		return nil, apiError(err)
	}
	return &types.ValidateTokenResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.ValidateTokenData{
			Valid:      result.GetValid(),
			UserID:     result.GetUserId(),
			Identifier: result.GetIdentifier(),
			ExpiresAt:  result.GetExpiresAt(),
		},
	}, nil
}
