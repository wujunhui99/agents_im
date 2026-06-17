package logic

import (
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/auth/rpc/internal/model"

	"golang.org/x/crypto/bcrypt"
)

// passwordAlgoDBBcrypt 是 auth_credentials.password_algo / code_hash_algo 的 bcrypt 取值。
// bcrypt 哈希自带盐与 cost，无需 salt 列。退役 internal/auth 后哈希实现统一在此。
const passwordAlgoDBBcrypt = model.PasswordAlgoBcrypt

// validatePassword 注册/凭据密码规则：非空、8~128 字符。
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

// hashPassword 生成 bcrypt 哈希（algo=passwordAlgoDBBcrypt）。
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", apperror.Internal("password hash failed")
	}
	return string(hash), nil
}

// verifyPassword 校验明文与存储哈希。Postgres 凭据全部为 bcrypt（algo=1）；
// auth_credentials 无 salt 列，旧 internal/auth 的 legacy sha256（需 salt）在 PG 路径
// 本就无法成立，故只支持 bcrypt，未知算法判定失败（拒绝登录）。
func verifyPassword(password string, hash string, algo int64) bool {
	switch algo {
	case passwordAlgoDBBcrypt:
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	default:
		return false
	}
}
