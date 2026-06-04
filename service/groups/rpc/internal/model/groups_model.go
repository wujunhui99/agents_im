package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ GroupsModel = (*customGroupsModel)(nil)

type (
	// GroupsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customGroupsModel.
	GroupsModel interface {
		groupsModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) GroupsModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// InsertGroup 插入一行 groups 并返回插入后的行（含时间戳）；group_id 由调用方提供。
		InsertGroup(ctx context.Context, data *Groups) (*Groups, error)
		// FindGroupsByMember 返回某用户作为 active 成员所属的群，按 updated_at 倒序。
		FindGroupsByMember(ctx context.Context, accountId string) ([]*Groups, error)
		// UpdateNameDescription 更新群名/描述并刷新 updated_at，返回更新后的行；群不存在返回 ErrNotFound。
		UpdateNameDescription(ctx context.Context, groupId, name, description string) (*Groups, error)
	}

	customGroupsModel struct {
		*defaultGroupsModel
	}
)

// NewGroupsModel returns a model for the database table.
func NewGroupsModel(conn sqlx.SqlConn) GroupsModel {
	return &customGroupsModel{
		defaultGroupsModel: newGroupsModel(conn),
	}
}

func (m *customGroupsModel) WithSession(session sqlx.Session) GroupsModel {
	return NewGroupsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customGroupsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customGroupsModel) InsertGroup(ctx context.Context, data *Groups) (*Groups, error) {
	query := fmt.Sprintf(`insert into %s (group_id, name, description, creator_account_id)
values ($1, $2, $3, $4)
returning %s`, m.table, groupsRows)
	var resp Groups
	err := m.conn.QueryRowCtx(ctx, &resp, query, data.GroupId, data.Name, data.Description, data.CreatorAccountId)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customGroupsModel) FindGroupsByMember(ctx context.Context, accountId string) ([]*Groups, error) {
	query := fmt.Sprintf(`select g.group_id, g.name, g.description, g.creator_account_id, g.created_at, g.updated_at
from %s g
join %s gm on gm.group_id = g.group_id
where gm.account_id = $1 and gm.status = $2
order by g.updated_at desc, g.group_id asc`, m.table, `"public"."group_members"`)
	var resp []*Groups
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, accountId, MemberStatusActive); err != nil {
		return nil, err
	}
	return resp, nil
}

func (m *customGroupsModel) UpdateNameDescription(ctx context.Context, groupId, name, description string) (*Groups, error) {
	query := fmt.Sprintf(`update %s
set name = $2, description = $3, updated_at = now()
where group_id = $1
returning %s`, m.table, groupsRows)
	var resp Groups
	err := m.conn.QueryRowCtx(ctx, &resp, query, groupId, name, description)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}
