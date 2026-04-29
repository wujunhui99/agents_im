package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/authpb"

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

func (l *ValidateTokenLogic) ValidateToken(in *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	result, err := l.svcCtx.AuthLogic.ValidateToken(l.ctx, business.ValidateTokenRequest{
		Token: in.GetToken(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toValidateTokenResponse(result), nil
}
