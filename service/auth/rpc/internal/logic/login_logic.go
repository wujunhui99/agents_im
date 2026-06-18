package logic

import (
	"context"
	"errors"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type LoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LoginLogic) Login(in *auth.LoginRequest) (*auth.AuthResponse, error) {
	identifier, err := normalizeIdentifier(in.GetIdentifier())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := validatePassword(in.GetPassword()); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	cred, err := l.svcCtx.Credentials.FindAuthByIdentifier(l.ctx, identifier)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, rpcerror.ToStatus(apperror.Unauthenticated("invalid identifier or password"))
		}
		return nil, rpcerror.ToStatus(err)
	}
	if !verifyPassword(in.GetPassword(), cred.PasswordHash, cred.PasswordAlgo) {
		return nil, rpcerror.ToStatus(apperror.Unauthenticated("invalid identifier or password"))
	}

	// 用户资料读经属主 user-rpc（注册/登录的控制流耦合，#551 既有 rpc 互调例外）。
	userResp, err := l.svcCtx.Users.GetUserByID(l.ctx, &userclient.GetUserByIDRequest{UserId: cred.AccountID})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}

	resp, err := issueToken(l.ctx, l.svcCtx, userResp.GetUser(), in.GetDevice(), in.GetLoginIp())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return resp, nil
}
