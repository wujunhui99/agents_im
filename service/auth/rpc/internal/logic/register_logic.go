package logic

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	business "github.com/wujunhui99/agents_im/service/auth/core/logic"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *RegisterLogic) Register(in *auth.RegisterRequest) (*auth.AuthResponse, error) {
	result, err := l.svcCtx.AuthLogic.Register(l.ctx, business.RegisterRequest{
		Identifier:            in.GetIdentifier(),
		Email:                 in.GetEmail(),
		EmailVerificationCode: in.GetEmailVerificationCode(),
		Password:              in.GetPassword(),
		DisplayName:           in.GetDisplayName(),
		Name:                  in.GetName(),
		Gender:                in.GetGender(),
		BirthDate:             in.GetBirthDate(),
		Region:                in.GetRegion(),
		Device:                in.GetDevice(),
		LoginIP:               in.GetLoginIp(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toAuthResponse(result), nil
}
