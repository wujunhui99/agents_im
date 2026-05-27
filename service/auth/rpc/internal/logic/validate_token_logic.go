package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
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
	result, err := l.svcCtx.AuthLogic.ValidateToken(l.ctx, business.ValidateTokenRequest{Token: in.GetToken()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toValidateTokenResponse(result), nil
}
