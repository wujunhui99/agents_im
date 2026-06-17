package logic

import (
	"context"

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

// ParseToken 与 ValidateToken 同义（校验签名 + 活跃会话）：保持既有 RPC 行为不变。
func (l *ParseTokenLogic) ParseToken(in *auth.ValidateTokenRequest) (*auth.ValidateTokenResponse, error) {
	claims, err := l.svcCtx.Tokens.Validate(in.GetToken())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := l.svcCtx.Sessions.Validate(l.ctx, claims.UserID, claims.Device, claims.JTI); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toValidateTokenResponse(claims), nil
}
