package model

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	pgUniqueViolationCode     = "23505"
	pgForeignKeyViolationCode = "23503"
)

var _ AuthCredentialsModel = (*customAuthCredentialsModel)(nil)

// CredentialAuth 是登录校验所需的凭据投影：经 accounts.identifier 关联取得。
// auth_credentials 不再存 email/identifier（#014 后归 accounts），登录后用 user-rpc 取资料。
type CredentialAuth struct {
	AccountID    string `db:"account_id"`
	PasswordHash string `db:"password_hash"`
	PasswordAlgo int64  `db:"password_algo"`
}

type (
	// AuthCredentialsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthCredentialsModel.
	AuthCredentialsModel interface {
		authCredentialsModel
		// UpsertPassword 创建或重置 account_id 对应凭据的密码（测试账户设置/重置密码用）。
		// 已有凭据时只更新 password_hash/password_algo；返回 created=true 表示新建、false 表示重置。
		UpsertPassword(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) (created bool, err error)
		// InsertPasswordIfAbsent 仅在 account_id 没有凭据时创建密码；已有凭据不覆盖。
		InsertPasswordIfAbsent(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) (created bool, err error)
		// InsertCredential 为注册新建凭据；account_id 已有凭据返回 AlreadyExists、account 不存在返回 NotFound。
		InsertCredential(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) error
		// FindAuthByIdentifier 经 accounts.identifier 取登录凭据；无记录返回 ErrNotFound。
		FindAuthByIdentifier(ctx context.Context, identifier string) (*CredentialAuth, error)
		// EmailExists 判断是否已有凭据绑定该规范化邮箱（注册前查重）。
		EmailExists(ctx context.Context, emailNormalized string) (bool, error)
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) AuthCredentialsModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排业务事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error
	}

	customAuthCredentialsModel struct {
		*defaultAuthCredentialsModel
	}
)

// NewAuthCredentialsModel returns a model for the database table.
func NewAuthCredentialsModel(conn sqlx.SqlConn) AuthCredentialsModel {
	return &customAuthCredentialsModel{
		defaultAuthCredentialsModel: newAuthCredentialsModel(conn),
	}
}

func (m *customAuthCredentialsModel) WithSession(session sqlx.Session) AuthCredentialsModel {
	return NewAuthCredentialsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAuthCredentialsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customAuthCredentialsModel) UpsertPassword(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) (bool, error) {
	// xmax = 0 仅在行由本语句新插入时成立，用于区分新建 / 重置。
	var created bool
	query := `insert into ` + m.table + ` (account_id, password_hash, password_algo)
values ($1, $2, $3)
on conflict (account_id) do update
set password_hash = excluded.password_hash,
    password_algo = excluded.password_algo,
    updated_at = now()
returning (xmax = 0) as created`
	if err := m.conn.QueryRowCtx(ctx, &created, query, accountID, passwordHash, passwordAlgo); err != nil {
		return false, err
	}
	return created, nil
}

func (m *customAuthCredentialsModel) InsertPasswordIfAbsent(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) (bool, error) {
	var created bool
	query := `insert into ` + m.table + ` (account_id, password_hash, password_algo)
values ($1, $2, $3)
on conflict (account_id) do nothing
returning true as created`
	if err := m.conn.QueryRowCtx(ctx, &created, query, accountID, passwordHash, passwordAlgo); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return created, nil
}

func (m *customAuthCredentialsModel) InsertCredential(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) error {
	query := `insert into ` + m.table + ` (account_id, password_hash, password_algo) values ($1, $2, $3)`
	if _, err := m.conn.ExecCtx(ctx, query, accountID, passwordHash, passwordAlgo); err != nil {
		if isPgUniqueViolation(err) {
			return apperror.AlreadyExists("auth credential already exists")
		}
		if isPgForeignKeyViolation(err) {
			return apperror.NotFound("account not found")
		}
		return err
	}
	return nil
}

func (m *customAuthCredentialsModel) FindAuthByIdentifier(ctx context.Context, identifier string) (*CredentialAuth, error) {
	var row CredentialAuth
	err := m.conn.QueryRowCtx(ctx, &row, `
select c.account_id, c.password_hash, c.password_algo
from `+m.table+` c
join "public"."accounts" a on a.account_id = c.account_id
where a.identifier = $1`, identifier)
	switch err {
	case nil:
		return &row, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customAuthCredentialsModel) EmailExists(ctx context.Context, emailNormalized string) (bool, error) {
	var accountID string
	err := m.conn.QueryRowCtx(ctx, &accountID, `
select c.account_id
from `+m.table+` c
join "public"."accounts" a on a.account_id = c.account_id
where a.email_normalized = $1 and a.email_normalized <> ''
limit 1`, emailNormalized)
	switch err {
	case nil:
		return true, nil
	case sqlx.ErrNotFound:
		return false, nil
	default:
		return false, err
	}
}

func isPgUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode
}

func isPgForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolationCode
}
