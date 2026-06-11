package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type EnsureTestCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnsureTestCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnsureTestCredentialLogic {
	return &EnsureTestCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EnsureTestCredential 为测试账户（account_type=test）创建/重置登录凭据。
// 账户本体由 user-rpc CreateTestAccount 先建，admin-api BFF 编排两步调用。
// 仅允许 test 账户：经 AccountsGuard 跨域鉴权读校验，防止被误用于覆盖
// 普通用户/管理员的密码。
func (l *EnsureTestCredentialLogic) EnsureTestCredential(in *auth.EnsureTestCredentialRequest) (*auth.EnsureTestCredentialResponse, error) {
	userID := strings.TrimSpace(in.GetUserId())
	if userID == "" {
		return nil, rpcerror.ToStatus(apperror.InvalidArgument("user_id is required"))
	}
	if err := validatePassword(in.GetPassword()); err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	accountType, err := l.svcCtx.AccountsGuard.FindAccountTypeByID(l.ctx, userID)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, rpcerror.ToStatus(apperror.NotFound("account not found"))
		}
		return nil, rpcerror.ToStatus(err)
	}
	if accountType != model.AccountTypeDBTest {
		return nil, rpcerror.ToStatus(apperror.Forbidden("credential ensure is only allowed for test accounts"))
	}

	hash, err := hashPassword(in.GetPassword())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	created, err := l.svcCtx.Credentials.UpsertPassword(l.ctx, userID, hash, passwordAlgoDBBcrypt)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	return &auth.EnsureTestCredentialResponse{
		UserId:     userID,
		Identifier: strings.TrimSpace(in.GetIdentifier()),
		Rotated:    !created,
	}, nil
}
