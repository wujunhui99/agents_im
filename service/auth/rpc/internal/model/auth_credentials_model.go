package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AuthCredentialsModel = (*customAuthCredentialsModel)(nil)

type (
	// AuthCredentialsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAuthCredentialsModel.
	AuthCredentialsModel interface {
		authCredentialsModel
		// UpsertPassword 创建或重置 account_id 对应凭据的密码（测试账户设置/重置密码用）。
		// 已有凭据时只更新 password_hash/password_algo；返回 created=true 表示新建、false 表示重置。
		UpsertPassword(ctx context.Context, accountID string, passwordHash string, passwordAlgo int64) (created bool, err error)
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
