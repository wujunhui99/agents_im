package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

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

func (l *RequestRegistrationEmailCodeLogic) RequestRegistrationEmailCode(in *auth.RegistrationEmailCodeRequest) (*auth.RegistrationEmailCodeResponse, error) {
	result, err := l.svcCtx.AuthLogic.RequestRegistrationEmailCode(l.ctx, business.RegistrationEmailCodeRequest{
		Email: in.GetEmail(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return &auth.RegistrationEmailCodeResponse{
		Email:         result.Email,
		ExpireMinutes: int32(result.ExpireMinutes),
	}, nil
}
