package model

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	pgUniqueViolationCode     = "23505"
	pgCheckViolationCode      = "23514"
	pgForeignKeyViolationCode = "23503"
)

func isPostgresCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

// IsUniqueViolation 报告 err 是否为 postgres 唯一约束冲突（如对端已开启托管触发的
// conversation_ai_hosting_one_enabled_owner_idx 冲突）。由 Logic/Store 层映射成业务错误。
func IsUniqueViolation(err error) bool {
	return isPostgresCode(err, pgUniqueViolationCode)
}

// IsCheckViolation 报告 err 是否为 postgres check 约束冲突（如非法 conversation_id/mode）。
// 由 Logic/Store 层映射成 apperror.InvalidArgument。
func IsCheckViolation(err error) bool {
	return isPostgresCode(err, pgCheckViolationCode)
}

// IsForeignKeyViolation 报告 err 是否为 postgres 外键约束冲突（如绑定引用不存在的 prompt/tool/mcp）。
// 由 Logic/Store 层映射成 apperror.NotFound。
func IsForeignKeyViolation(err error) bool {
	return isPostgresCode(err, pgForeignKeyViolationCode)
}
