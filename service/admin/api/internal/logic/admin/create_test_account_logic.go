// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/rpcerror"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/svc"
	"github.com/wujunhui99/agents_im/service/admin/api/internal/types"
	authpb "github.com/wujunhui99/agents_im/service/auth/rpc/auth"
	userpb "github.com/wujunhui99/agents_im/service/user/rpc/user"

	"github.com/zeromicro/go-zero/core/logx"
)

// generatedPasswordLength 自动生成密码的长度；字符集去掉了易混淆字符（0O1lI）。
const generatedPasswordLength = 12

const generatedPasswordCharset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// rfc3339FromUnixMilli 把 user-rpc 的 UnixMilli 时间戳渲染成对外 RFC3339(UTC) 串；0 → 空串
// （与旧 user-rpc formatTime 的零值行为一致，AdminUser.CreatedAt/UpdatedAt 仍是 string）。
func rfc3339FromUnixMilli(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

type CreateTestAccountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateTestAccountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateTestAccountLogic {
	return &CreateTestAccountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateTestAccount BFF 编排创建测试账户（不绑定邮箱）：
// 1. user-rpc CreateTestAccount 建号（account_type=test；幂等，已存在的 test 账户直接返回）；
// 2. auth-rpc EnsureTestCredential 设置/重置登录凭据（密码缺省时此处自动生成）。
// 同名重复创建 = 重置该测试账户密码。生效密码仅本次响应返回。
func (l *CreateTestAccountLogic) CreateTestAccount(req *types.AdminTestAccountCreateReq) (*types.AdminTestAccountCreateResp, error) {
	identifier := strings.TrimSpace(req.Identifier)
	if identifier == "" {
		return nil, apperror.InvalidArgument("identifier is required")
	}

	password := req.Password
	if strings.TrimSpace(password) == "" {
		generated, err := generatePassword(generatedPasswordLength)
		if err != nil {
			return nil, apperror.Internal("generate password failed")
		}
		password = generated
	}

	created, err := l.svcCtx.UserRPC.CreateTestAccount(l.ctx, &userpb.CreateTestAccountRequest{
		Identifier:  identifier,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		return nil, rpcerror.FromStatus(err)
	}
	user := created.GetUser()

	if _, err := l.svcCtx.AuthRPC.EnsureTestCredential(l.ctx, &authpb.EnsureTestCredentialRequest{
		UserId:     user.GetUserId(),
		Identifier: user.GetIdentifier(),
		Password:   password,
	}); err != nil {
		return nil, rpcerror.FromStatus(err)
	}

	return &types.AdminTestAccountCreateResp{
		Code:    codeOK,
		Message: messageOK,
		Data: types.AdminTestAccountData{
			User: types.AdminUser{
				UserID:        user.GetUserId(),
				AccountID:     user.GetUserId(),
				Identifier:    user.GetIdentifier(),
				DisplayName:   user.GetDisplayName(),
				Name:          user.GetName(),
				Gender:        user.GetGender(),
				BirthDate:     user.GetBirthDate(),
				Region:        user.GetRegion(),
				AccountType:   user.GetAccountType(),
				AvatarMediaID: user.GetAvatarMediaId(),
				AvatarURL:     user.GetAvatarUrl(),
				CreatedAt:     rfc3339FromUnixMilli(user.GetCreatedAt()),
				UpdatedAt:     rfc3339FromUnixMilli(user.GetUpdatedAt()),
			},
			Password:       password,
			AlreadyExisted: created.GetAlreadyExists(),
		},
	}, nil
}

func generatePassword(length int) (string, error) {
	charset := []rune(generatedPasswordCharset)
	out := make([]rune, length)
	for i := range out {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		out[i] = charset[idx.Int64()]
	}
	return string(out), nil
}
