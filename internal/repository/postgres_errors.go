package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	pgUniqueViolationCode     = "23505"
	pgForeignKeyViolationCode = "23503"
	pgCheckViolationCode      = "23514"
)

func isNotFound(err error) bool {
	return errors.Is(err, sqlx.ErrNotFound)
}

func isPostgresCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func isPostgresUniqueViolation(err error) bool {
	return isPostgresCode(err, pgUniqueViolationCode)
}

func isPostgresForeignKeyViolation(err error) bool {
	return isPostgresCode(err, pgForeignKeyViolationCode)
}

func isPostgresCheckViolation(err error) bool {
	return isPostgresCode(err, pgCheckViolationCode)
}
