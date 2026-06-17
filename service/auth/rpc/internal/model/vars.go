package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var ErrNotFound = sqlx.ErrNotFound

// auth 域 DB 整型编码单一来源（smallint）。
const (
	// EmailVerificationPurposeRegister 是 auth_email_verification_tokens.purpose 的注册取值。
	EmailVerificationPurposeRegister int64 = 1
	// PasswordAlgoBcrypt 是 password_algo / code_hash_algo 的 bcrypt 取值。
	PasswordAlgoBcrypt int64 = 1
)
