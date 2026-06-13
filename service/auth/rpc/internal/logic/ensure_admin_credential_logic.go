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

type EnsureAdminCredentialLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewEnsureAdminCredentialLogic(ctx context.Context, svcCtx *svc.ServiceContext) *EnsureAdminCredentialLogic {
	return &EnsureAdminCredentialLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// EnsureAdminCredential 为管理员账号补齐首次登录凭据。
func (l *EnsureAdminCredentialLogic) EnsureAdminCredential(in *auth.EnsureAdminCredentialRequest) (*auth.EnsureAdminCredentialResponse, error) {
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
	if accountType != model.AccountTypeDBAdmin {
		return nil, rpcerror.ToStatus(apperror.Forbidden("credential ensure is only allowed for admin accounts"))
	}

	hash, err := hashPassword(in.GetPassword())
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}
	created, err := l.svcCtx.Credentials.InsertPasswordIfAbsent(l.ctx, userID, hash, passwordAlgoDBBcrypt)
	if err != nil {
		return nil, rpcerror.ToStatus(err)
	}

	return &auth.EnsureAdminCredentialResponse{
		UserId:     userID,
		Identifier: strings.TrimSpace(in.GetIdentifier()),
		Created:    created,
	}, nil
}
