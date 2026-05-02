package logic

import (
	"context"

	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	"github.com/wujunhui99/agents_im/internal/rpcgen/auth/internal/svc"
	"github.com/wujunhui99/agents_im/internal/rpcgen/rpcerror"
	"github.com/wujunhui99/agents_im/proto/authpb"

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

func (l *RegisterLogic) Register(in *authpb.RegisterRequest) (*authpb.AuthResponse, error) {
	result, err := l.svcCtx.AuthLogic.Register(l.ctx, business.RegisterRequest{
		Identifier:  in.GetIdentifier(),
		Password:    in.GetPassword(),
		DisplayName: in.GetDisplayName(),
		Name:        in.GetName(),
		Gender:      in.GetGender(),
		BirthDate:   in.GetBirthDate(),
		Region:      in.GetRegion(),
	})
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return toAuthResponse(result), nil
}
