package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/authpb"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestRegistrationEmailCodeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRequestRegistrationEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestRegistrationEmailCodeLogic {
	return &RequestRegistrationEmailCodeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RequestRegistrationEmailCodeLogic) RequestRegistrationEmailCode(in *authpb.RegistrationEmailCodeRequest) (*authpb.RegistrationEmailCodeResponse, error) {
	result, err := l.svcCtx.AuthLogic.RequestRegistrationEmailCode(l.ctx, business.RegistrationEmailCodeRequest{
		Email: in.GetEmail(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &authpb.RegistrationEmailCodeResponse{
		Email:         result.Email,
		ExpireMinutes: int32(result.ExpireMinutes),
	}, nil
}
