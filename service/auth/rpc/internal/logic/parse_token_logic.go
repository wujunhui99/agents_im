package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ParseTokenLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewParseTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ParseTokenLogic {
	return &ParseTokenLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ParseTokenLogic) ParseToken(in *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	result, err := l.svcCtx.AuthLogic.ValidateToken(l.ctx, business.ValidateTokenRequest{Token: in.GetToken()})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toValidateTokenResponse(result), nil
}
