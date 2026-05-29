// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package auth

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	business "github.com/wujunhui99/agents_im/internal/auth/logic"
	authsvc "github.com/wujunhui99/agents_im/internal/servicecontext/auth"
	"github.com/wujunhui99/agents_im/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RequestRegistrationEmailCodeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *authsvc.ServiceContext
}

func NewRequestRegistrationEmailCodeLogic(ctx context.Context, svcCtx *authsvc.ServiceContext) *RequestRegistrationEmailCodeLogic {
	return &RequestRegistrationEmailCodeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RequestRegistrationEmailCodeLogic) RequestRegistrationEmailCode(req *types.RegistrationEmailCodeReq) (resp *types.RegistrationEmailCodeResp, err error) {
	result, err := l.svcCtx.AuthLogic.RequestRegistrationEmailCode(l.ctx, business.RegistrationEmailCodeRequest{
		Email: req.Email,
	})
	if err != nil {
		return nil, err
	}
	return &types.RegistrationEmailCodeResp{
		Code:    string(apperror.CodeOK),
		Message: "ok",
		Data: types.RegistrationEmailCodeData{
			Email:         result.Email,
			ExpireMinutes: result.ExpireMinutes,
		},
	}, nil
}
