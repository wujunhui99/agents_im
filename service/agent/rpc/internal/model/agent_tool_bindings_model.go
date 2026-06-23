package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentToolBindingsModel = (*customAgentToolBindingsModel)(nil)

type (
	// AgentToolBindingsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentToolBindingsModel.
	AgentToolBindingsModel interface {
		agentToolBindingsModel
		withSession(session sqlx.Session) AgentToolBindingsModel

		// FindByAgentId 列出某 agent 的全部 tool 绑定，按 tool_id 稳定排序。
		FindByAgentId(ctx context.Context, agentId int64) ([]*AgentToolBindings, error)
	}

	customAgentToolBindingsModel struct {
		*defaultAgentToolBindingsModel
	}
)

func (m *customAgentToolBindingsModel) FindByAgentId(ctx context.Context, agentId int64) ([]*AgentToolBindings, error) {
	query := fmt.Sprintf("select %s from %s where agent_id = $1 order by tool_id", agentToolBindingsRows, m.table)
	var resp []*AgentToolBindings
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, agentId); err != nil {
		return nil, err
	}
	return resp, nil
}

// NewAgentToolBindingsModel returns a model for the database table.
func NewAgentToolBindingsModel(conn sqlx.SqlConn) AgentToolBindingsModel {
	return &customAgentToolBindingsModel{
		defaultAgentToolBindingsModel: newAgentToolBindingsModel(conn),
	}
}

func (m *customAgentToolBindingsModel) withSession(session sqlx.Session) AgentToolBindingsModel {
	return NewAgentToolBindingsModel(sqlx.NewSqlConnFromSession(session))
}
