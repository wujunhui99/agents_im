package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentsModel = (*customAgentsModel)(nil)

type (
	// AgentsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentsModel.
	AgentsModel interface {
		agentsModel

		// InsertReturning 插入一行并返回数据库生成的完整行（agent_id 自增、created_at/updated_at）。
		InsertReturning(ctx context.Context, data *Agents) (*Agents, error)
		// ListFiltered 按 status / createdBy（任一为空则不过滤）分页列出 agents，按 created_at,agent_id 升序。
		ListFiltered(ctx context.Context, status, createdBy string, limit, offset int) ([]*Agents, error)
		// UpdateNameDescription 局部更新 name/description（nil 跳过）并返回更新后的行；无变更时返回当前行。
		UpdateNameDescription(ctx context.Context, agentID int64, name, description *string) (*Agents, error)
		// UpdateStatus 更新 status 并返回更新后的行。
		UpdateStatus(ctx context.Context, agentID int64, status string) (*Agents, error)
	}

	customAgentsModel struct {
		*defaultAgentsModel
	}
)

// NewAgentsModel returns a model for the database table.
func NewAgentsModel(conn sqlx.SqlConn) AgentsModel {
	return &customAgentsModel{
		defaultAgentsModel: newAgentsModel(conn),
	}
}

const agentsReturning = "agent_id, account_id, name, description, status, created_by, created_at, updated_at"

func (m *customAgentsModel) InsertReturning(ctx context.Context, data *Agents) (*Agents, error) {
	query := fmt.Sprintf(`insert into %s (account_id, name, description, status, created_by)
values ($1, $2, $3, $4, $5)
returning %s`, m.table, agentsReturning)
	var resp Agents
	if err := m.conn.QueryRowCtx(ctx, &resp, query, data.AccountId, data.Name, data.Description, data.Status, data.CreatedBy); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *customAgentsModel) ListFiltered(ctx context.Context, status, createdBy string, limit, offset int) ([]*Agents, error) {
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 4)
	if status != "" {
		args = append(args, status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if createdBy != "" {
		args = append(args, createdBy)
		clauses = append(clauses, fmt.Sprintf("created_by = $%d", len(args)))
	}
	query := fmt.Sprintf("select %s from %s", agentsReturning, m.table)
	if len(clauses) > 0 {
		query += " where " + strings.Join(clauses, " and ")
	}
	query += " order by created_at asc, agent_id asc"
	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" limit $%d", len(args))
	}
	if offset > 0 {
		args = append(args, offset)
		query += fmt.Sprintf(" offset $%d", len(args))
	}
	var rows []*Agents
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *customAgentsModel) UpdateNameDescription(ctx context.Context, agentID int64, name, description *string) (*Agents, error) {
	setters := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if name != nil {
		args = append(args, *name)
		setters = append(setters, fmt.Sprintf("name = $%d", len(args)))
	}
	if description != nil {
		args = append(args, *description)
		setters = append(setters, fmt.Sprintf("description = $%d", len(args)))
	}
	if len(setters) == 0 {
		return m.FindOne(ctx, agentID)
	}
	args = append(args, agentID)
	query := fmt.Sprintf(`update %s set %s, updated_at = now() where agent_id = $%d returning %s`,
		m.table, strings.Join(setters, ", "), len(args), agentsReturning)
	var resp Agents
	if err := m.conn.QueryRowCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &resp, nil
}

func (m *customAgentsModel) UpdateStatus(ctx context.Context, agentID int64, status string) (*Agents, error) {
	query := fmt.Sprintf(`update %s set status = $2, updated_at = now() where agent_id = $1 returning %s`, m.table, agentsReturning)
	var resp Agents
	if err := m.conn.QueryRowCtx(ctx, &resp, query, agentID, status); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &resp, nil
}
