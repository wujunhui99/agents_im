package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/svc"
	"github.com/wujunhui99/agents_im/proto/authpb"

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

func (l *ParseTokenLogic) ParseToken(in *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	// todo: add your logic here and delete this line

	return &authpb.ValidateTokenResponse{}, nil
}
