package logic

import (
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"

	"golang.org/x/crypto/bcrypt"
)

// passwordAlgoDBBcrypt 是 auth_credentials.password_algo 的 bcrypt 取值，与
// internal/auth 的 `bcrypt-v1` 契约一致：登录校验端（internal/auth/logic 的
// BcryptPasswordHasher）按 algo=1 用 bcrypt.CompareHashAndPassword 验证。
// bcrypt 哈希自带盐与 cost，无需 salt 列。auth 域整体重构（退役 internal/auth）
// 时哈希实现统一迁入此处。
const passwordAlgoDBBcrypt int64 = 1

// validatePassword 与 internal/auth/logic 的注册密码规则一致：非空、8~128 字符。
func validatePassword(password string) error {
	if strings.TrimSpace(password) == "" {
		return apperror.InvalidArgument("password is required")
	}
	length := len([]rune(password))
	if length < 8 || length > 128 {
		return apperror.InvalidArgument("password must be 8 to 128 characters")
	}
	return nil
}

// hashPassword 生成与登录校验兼容的 bcrypt 哈希（algo=passwordAlgoDBBcrypt）。
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", apperror.Internal("password hash failed")
	}
	return string(hash), nil
}
