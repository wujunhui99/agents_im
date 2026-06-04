package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GroupMembersModel = (*customGroupMembersModel)(nil)

type (
	// GroupMembersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGroupMembersModel.
	GroupMembersModel interface {
		groupMembersModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) GroupMembersModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// FindActiveByGroup 返回某群所有 active 成员，按 account_id 升序。
		FindActiveByGroup(ctx context.Context, groupId string) ([]*GroupMembers, error)
		// UpsertActiveMember 新增或复活成员为 active（曾退群者 join_time 重置、left_at 清空）。
		UpsertActiveMember(ctx context.Context, groupId, accountId string, role int64) (*GroupMembers, error)
		// SetMemberLeft 把一个 active 成员置为 left（退群/被踢）；无 active 行返回 ErrNotFound。
		SetMemberLeft(ctx context.Context, groupId, accountId string) (*GroupMembers, error)
	}

	customGroupMembersModel struct {
		*defaultGroupMembersModel
	}
)

// NewGroupMembersModel returns a model for the database table.
func NewGroupMembersModel(conn sqlx.SqlConn) GroupMembersModel {
	return &customGroupMembersModel{
		defaultGroupMembersModel: newGroupMembersModel(conn),
	}
}

func (m *customGroupMembersModel) WithSession(session sqlx.Session) GroupMembersModel {
	return NewGroupMembersModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customGroupMembersModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customGroupMembersModel) FindActiveByGroup(ctx context.Context, groupId string) ([]*GroupMembers, error) {
	query := fmt.Sprintf(`select %s from %s where group_id = $1 and status = $2 order by account_id asc`, groupMembersRows, m.table)
	var resp []*GroupMembers
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, groupId, MemberStatusActive); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customGroupMembersModel) UpsertActiveMember(ctx context.Context, groupId, accountId string, role int64) (*GroupMembers, error) {
	query := fmt.Sprintf(`insert into %s (group_id, account_id, role, status)
values ($1, $2, $3, $4)
on conflict (group_id, account_id) do update
set role = excluded.role,
    status = excluded.status,
    join_time = now(),
    left_at = null,
    updated_at = now()
returning %s`, m.table, groupMembersRows)
	var resp GroupMembers
	err := m.conn.QueryRowCtx(ctx, &resp, query, groupId, accountId, role, MemberStatusActive)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customGroupMembersModel) SetMemberLeft(ctx context.Context, groupId, accountId string) (*GroupMembers, error) {
	query := fmt.Sprintf(`update %s
set status = $3, left_at = now(), updated_at = now()
where group_id = $1 and account_id = $2 and status = $4
returning %s`, m.table, groupMembersRows)
	var resp GroupMembers
	err := m.conn.QueryRowCtx(ctx, &resp, query, groupId, accountId, MemberStatusLeft, MemberStatusActive)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
