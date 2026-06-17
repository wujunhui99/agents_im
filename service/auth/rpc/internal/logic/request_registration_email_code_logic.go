package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"
	mailpb "github.com/wujunhui99/agents_im/service/third/rpc/mail"

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
	if l.svcCtx.Mailer == nil {
		return nil, rpcerror.ToStatus(apperror.ServiceUnavailable("mail service is not configured"))
	}
	email, err := normalizeEmail(in.GetEmail())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	now := time.Now().UTC()
	latest, err := l.svcCtx.EmailVerifications.Latest(l.ctx, model.EmailVerificationPurposeRegister, email)
	switch {
	case err == nil:
		if latest.LastSentAt.After(now.Add(-registrationSendCooldown)) {
			return nil, rpcerror.ToStatus(apperror.RateLimited("registration email code was sent too recently"))
		}
	case errors.Is(err, model.ErrNotFound):
		// 无历史 token，继续发送。
	default:
		return nil, rpcerror.ToStatus(err)
	}

	code, err := generateNumericRegistrationCode()
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("generate verification code failed"))
	}
	if err := validateVerificationCodeFormat(code); err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("generate verification code failed"))
	}
	codeHash, err := hashPassword(code)
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("hash verification code failed"))
	}
	tokenID, err := randomTokenID()
	if err != nil {
		return nil, rpcerror.ToStatus(apperror.Internal("generate verification token failed"))
	}
	expireMinutes := int(registrationCodeTTL / time.Minute)
	if expireMinutes <= 0 {
		expireMinutes = 1
	}

	if _, err := l.svcCtx.Mailer.SendTemplateEmail(l.ctx, &mailpb.SendTemplateEmailRequest{
		Recipients: []string{email},
		TemplateId: registrationEmailTemplate,
		Subject:    registrationEmailSubject,
		TemplateData: map[string]string{
			"code":           code,
			"expire_minutes": fmt.Sprintf("%d", expireMinutes),
		},
		IdempotencyKey: "auth-register-email-code-" + tokenID,
	}); err != nil {
		logx.WithContext(l.ctx).Errorf("send registration verification email failed: %v", err)
		return nil, rpcerror.ToStatus(apperror.ServiceUnavailable("mail service is unavailable"))
	}

	if err := l.svcCtx.EmailVerifications.SupersedeAndInsert(l.ctx, &model.AuthEmailVerificationTokens{
		Id:              tokenID,
		Purpose:         model.EmailVerificationPurposeRegister,
		EmailNormalized: email,
		CodeHash:        codeHash,
		CodeHashAlgo:    model.PasswordAlgoBcrypt,
		ExpiresAt:       now.Add(registrationCodeTTL),
		LastSentAt:      now,
		CreatedAt:       now,
	}); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	return &auth.RegistrationEmailCodeResponse{
		Email:         email,
		ExpireMinutes: int32(expireMinutes),
	}, nil
}
