package useradapter

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// useradapter 定义 auth 对「用户域」的依赖契约（接口 + DTO）。实现由 auth-rpc 注入：
// #551 起走 service/auth/rpc/internal/userrpc（user-rpc 客户端），不再绑 internal/logic.UserLogic。

type ExistsResult struct {
	Identifier string
	Exists     bool
}

type CreateUserRequest struct {
	Identifier      string
	Email           string
	EmailVerifiedAt time.Time
	DisplayName     string
	Name            string
	Gender          string
	BirthDate       string
	Region          string
}

type UserProfile struct {
	UserID          string
	Identifier      string
	Email           string
	EmailVerifiedAt time.Time
	DisplayName     string
	Name            string
	Gender          string
	BirthDate       string
	Region          string
	AccountType     string
	AvatarMediaID   string
	AvatarURL       string
	CreatedAt       string
	UpdatedAt       string
}

type UserClient interface {
	ExistsByIdentifier(ctx context.Context, identifier string) (ExistsResult, error)
	CreateUser(ctx context.Context, req CreateUserRequest) (UserProfile, error)
	GetUserByID(ctx context.Context, userID string) (UserProfile, error)
}

// NormalizeIdentifier 校验并规范化登录标识符。原先转调 internal/logic.NormalizeIdentifier，
// #551 内联以彻底脱离 internal/logic（规则同步：小写、trim、3-32 长度、首字符字母/数字、
// 仅字母数字下划线）。
func NormalizeIdentifier(identifier string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(identifier))
	if len(normalized) < 3 || len(normalized) > 32 {
		return "", apperror.InvalidArgument("identifier must be 3 to 32 characters")
	}
	for idx, r := range normalized {
		isLetter := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isUnderscore := r == '_'
		if idx == 0 && !isLetter && !isDigit {
			return "", apperror.InvalidArgument("identifier must start with a letter or digit")
		}
		if !isLetter && !isDigit && !isUnderscore {
			return "", apperror.InvalidArgument("identifier can only contain letters, digits, and underscore")
		}
	}
	return normalized, nil
}
