package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ FriendshipsModel = (*customFriendshipsModel)(nil)

type (
	// FriendshipsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customFriendshipsModel.
	FriendshipsModel interface {
		friendshipsModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) FriendshipsModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排业务事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// FindPairForUpdate 在事务内取 account->friend 单向关系并加行锁；不存在返回 ErrNotFound。
		FindPairForUpdate(ctx context.Context, accountID, friendID string) (*Friendships, error)
		// UpsertStatus 新增或覆盖 account->friend 单向关系为指定 status（created_at/updated_at 重置为 now）。
		UpsertStatus(ctx context.Context, accountID, friendID string, status int64) (*Friendships, error)
		// EnsureAccepted 幂等地把 account->friend 单向关系置为 accepted：不存在则插入，存在则只更新
		// status/updated_at（保留 created_at），供 EnsureFriendship 重复调用不重置好友建立时间。
		EnsureAccepted(ctx context.Context, accountID, friendID string) (*Friendships, error)
		// ListByAccountStatus 返回某账号作为发起方、处于指定 status 的关系，按 friend_account_id 升序。
		ListByAccountStatus(ctx context.Context, accountID string, status int64) ([]*Friendships, error)
		// ListByFriendStatus 返回某账号作为接收方、处于指定 status 的关系，按 account_id 升序。
		ListByFriendStatus(ctx context.Context, friendID string, status int64) ([]*Friendships, error)
		// MarkAcceptedDeleted 把一条 accepted 的 account->friend 关系置为 deleted 并返回该行；无 accepted 行返回 ErrNotFound。
		MarkAcceptedDeleted(ctx context.Context, accountID, friendID string) (*Friendships, error)
		// MarkAcceptedDeletedSilent 把反向 accepted 关系置为 deleted（无则忽略），不返回行。
		MarkAcceptedDeletedSilent(ctx context.Context, accountID, friendID string) error
	}

	customFriendshipsModel struct {
		*defaultFriendshipsModel
	}
)

// NewFriendshipsModel returns a model for the database table.
func NewFriendshipsModel(conn sqlx.SqlConn) FriendshipsModel {
	return &customFriendshipsModel{
		defaultFriendshipsModel: newFriendshipsModel(conn),
	}
}

func (m *customFriendshipsModel) WithSession(session sqlx.Session) FriendshipsModel {
	return NewFriendshipsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customFriendshipsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customFriendshipsModel) FindPairForUpdate(ctx context.Context, accountID, friendID string) (*Friendships, error) {
	query := fmt.Sprintf("select %s from %s where account_id = $1 and friend_account_id = $2 limit 1 for update", friendshipsRows, m.table)
	var resp Friendships
	err := m.conn.QueryRowCtx(ctx, &resp, query, accountID, friendID)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customFriendshipsModel) UpsertStatus(ctx context.Context, accountID, friendID string, status int64) (*Friendships, error) {
	query := fmt.Sprintf(`insert into %s (account_id, friend_account_id, status)
values ($1, $2, $3)
on conflict (account_id, friend_account_id) do update
set status = excluded.status,
    created_at = now(),
    updated_at = now()
returning %s`, m.table, friendshipsRows)
	var resp Friendships
	if err := m.conn.QueryRowCtx(ctx, &resp, query, accountID, friendID, status); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *customFriendshipsModel) EnsureAccepted(ctx context.Context, accountID, friendID string) (*Friendships, error) {
	query := fmt.Sprintf(`insert into %s (account_id, friend_account_id, status)
values ($1, $2, $3)
on conflict (account_id, friend_account_id) do update
set status = excluded.status,
    updated_at = now()
returning %s`, m.table, friendshipsRows)
	var resp Friendships
	if err := m.conn.QueryRowCtx(ctx, &resp, query, accountID, friendID, FriendshipStatusAccepted); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *customFriendshipsModel) ListByAccountStatus(ctx context.Context, accountID string, status int64) ([]*Friendships, error) {
	query := fmt.Sprintf("select %s from %s where account_id = $1 and status = $2 order by friend_account_id asc", friendshipsRows, m.table)
	var resp []*Friendships
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, accountID, status); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customFriendshipsModel) ListByFriendStatus(ctx context.Context, friendID string, status int64) ([]*Friendships, error) {
	query := fmt.Sprintf("select %s from %s where friend_account_id = $1 and status = $2 order by account_id asc", friendshipsRows, m.table)
	var resp []*Friendships
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, friendID, status); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customFriendshipsModel) MarkAcceptedDeleted(ctx context.Context, accountID, friendID string) (*Friendships, error) {
	query := fmt.Sprintf(`update %s
set status = $3, updated_at = now()
where account_id = $1 and friend_account_id = $2 and status = $4
returning %s`, m.table, friendshipsRows)
	var resp Friendships
	err := m.conn.QueryRowCtx(ctx, &resp, query, accountID, friendID, FriendshipStatusDeleted, FriendshipStatusAccepted)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customFriendshipsModel) MarkAcceptedDeletedSilent(ctx context.Context, accountID, friendID string) error {
	query := fmt.Sprintf(`update %s
set status = $3, updated_at = now()
where account_id = $1 and friend_account_id = $2 and status = $4`, m.table)
	_, err := m.conn.ExecCtx(ctx, query, accountID, friendID, FriendshipStatusDeleted, FriendshipStatusAccepted)
	return err
}
