package logic

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	stdmail "net/mail"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// 注册邮箱验证码相关常量（沿用 internal/auth 既有契约）。
const (
	registrationCodeTTL       = 10 * time.Minute
	registrationSendCooldown  = time.Minute
	maxVerificationAttempts   = 5
	registrationEmailTemplate = 177952
	registrationEmailSubject  = "AgenticIM 注册验证码"
	registrationCodeLength    = 6
)

// normalizeEmail 校验并规范化邮箱（小写、trim、合法地址且含 @）。
func normalizeEmail(email string) (string, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return "", apperror.InvalidArgument("email is required")
	}
	address, err := stdmail.ParseAddress(email)
	if err != nil || strings.TrimSpace(address.Address) == "" {
		return "", apperror.InvalidArgument("email is invalid")
	}
	normalized := strings.ToLower(strings.TrimSpace(address.Address))
	if !strings.Contains(normalized, "@") {
		return "", apperror.InvalidArgument("email is invalid")
	}
	return normalized, nil
}

// normalizeIdentifier 校验并规范化登录标识符（小写、trim、3-32 长度、首字符字母/数字、
// 仅字母数字下划线）。内联自旧 internal/auth/useradapter.NormalizeIdentifier。
func normalizeIdentifier(identifier string) (string, error) {
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

// validateVerificationCodeFormat 校验验证码为 6 位数字。
func validateVerificationCodeFormat(code string) error {
	if len(code) != registrationCodeLength {
		return apperror.InvalidArgument("email verification code is invalid or expired")
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return apperror.InvalidArgument("email verification code is invalid or expired")
		}
	}
	return nil
}

// generateNumericRegistrationCode 生成 6 位随机数字验证码。
func generateNumericRegistrationCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", value.Int64()), nil
}

// randomTokenID 生成 16 字节随机十六进制 token id。
func randomTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// formatTime 把时间格式化为 RFC3339(UTC)；零值返回空串。
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
