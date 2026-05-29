// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/auth/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestRegistrationEmailCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRequestRegistrationEmailCodeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RequestRegistrationEmailCodeLogic {
	return &RequestRegistrationEmailCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RequestRegistrationEmailCodeLogic) RequestRegistrationEmailCode(req *types.RegistrationEmailCodeReq) (resp *types.RegistrationEmailCodeResp, err error) {
	result, err := l.svcCtx.AuthRPC.RequestRegistrationEmailCode(l.ctx, &authpb.RegistrationEmailCodeRequest{
		Email: req.Email,
	})
	if err != nil {
		return nil, apiError(err)
	}
	return &types.RegistrationEmailCodeResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.RegistrationEmailCodeData{
			Email:         result.GetEmail(),
			ExpireMinutes: int(result.GetExpireMinutes()),
		},
	}, nil
}
