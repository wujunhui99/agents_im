package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ValidateTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewValidateTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ValidateTokenLogic {
	return &ValidateTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ValidateTokenLogic) ValidateToken(in *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	claims, err := l.svcCtx.Tokens.Validate(in.GetToken())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := l.svcCtx.Sessions.Validate(l.ctx, claims.UserID, claims.Device, claims.JTI); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toValidateTokenResponse(claims), nil
}
