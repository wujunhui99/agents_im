package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	auth "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"
	"github.com/wujunhui99/agents_im/service/user/rpc/userclient"

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
	if err := validatePassword(in.GetPassword()); err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	email, err := normalizeEmail(in.GetEmail())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	code := strings.TrimSpace(in.GetEmailVerificationCode())
	if code == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("email verification code is required"))
	}

	// 标识符查重经属主 user-rpc（注册必须建 user，#551 既有 rpc 互调例外）。
	exists, err := l.svcCtx.Users.ExistsByIdentifier(l.ctx, &userclient.ExistsByIdentifierRequest{Identifier: in.GetIdentifier()})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	if exists.GetExists() {
		return nil, rpcerror.ToStatus(apperror.AlreadyExists("identifier already exists"))
	}

	verifiedAt, err := l.consumeRegistrationEmailCode(email, code)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := l.ensureEmailAvailable(email); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	created, err := l.svcCtx.Users.CreateUser(l.ctx, &userclient.CreateUserRequest{
		Identifier:      exists.GetIdentifier(),
		Email:           email,
		EmailVerifiedAt: formatTime(verifiedAt),
		DisplayName:     in.GetDisplayName(),
		Name:            in.GetName(),
		Gender:          in.GetGender(),
		BirthDate:       in.GetBirthDate(),
		Region:          in.GetRegion(),
		// account_type 留空 = user（user-rpc 默认）；auth 注册只建普通用户。
	})
	if err != nil {
		return nil, rpcerror.ToStatus(rpcerror.FromStatus(err))
	}
	user := created.GetUser()

	hash, err := hashPassword(in.GetPassword())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	if err := l.svcCtx.Credentials.InsertCredential(l.ctx, user.GetUserId(), hash, passwordAlgoDBBcrypt); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	resp, err := issueToken(l.ctx, l.svcCtx, user, in.GetDevice(), in.GetLoginIp())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	return resp, nil
}

// ensureEmailAvailable 注册前查邮箱是否已被现存凭据占用。
func (l *RegisterLogic) ensureEmailAvailable(email string) error {
	exists, err := l.svcCtx.Credentials.EmailExists(l.ctx, email)
	if err != nil {
		return err
	}
	if exists {
		return apperror.AlreadyExists("email already exists")
	}
	return nil
}

// consumeRegistrationEmailCode 校验并消费注册邮箱验证码，返回验证完成时间。
func (l *RegisterLogic) consumeRegistrationEmailCode(email string, code string) (time.Time, error) {
	if err := validateVerificationCodeFormat(code); err != nil {
		return time.Time{}, err
	}
	now := time.Now().UTC()
	verification, err := l.svcCtx.EmailVerifications.Latest(l.ctx, model.EmailVerificationPurposeRegister, email)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
		}
		return time.Time{}, err
	}
	if verification.ConsumedAt.Valid || !now.Before(verification.ExpiresAt) {
		return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
	}
	if verification.AttemptCount >= maxVerificationAttempts {
		return time.Time{}, apperror.RateLimited("email verification attempts exceeded")
	}
	if !verifyPassword(code, verification.CodeHash, verification.CodeHashAlgo) {
		attempts, err := l.svcCtx.EmailVerifications.IncrementAttempts(l.ctx, verification.Id, now)
		if err != nil {
			return time.Time{}, err
		}
		if attempts >= maxVerificationAttempts {
			return time.Time{}, apperror.RateLimited("email verification attempts exceeded")
		}
		return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
	}
	consumedAt, err := l.svcCtx.EmailVerifications.Consume(l.ctx, verification.Id, now)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return time.Time{}, apperror.InvalidArgument("email verification code is invalid or expired")
		}
		return time.Time{}, err
	}
	return consumedAt, nil
}
