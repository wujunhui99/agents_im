package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AgentToolsModel = (*customAgentToolsModel)(nil)

type (
	// AgentToolsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentToolsModel.
	AgentToolsModel interface {
		agentToolsModel
		withSession(session sqlx.Session) AgentToolsModel
	}

	customAgentToolsModel struct {
		*defaultAgentToolsModel
	}
)

// NewAgentToolsModel returns a model for the database table.
func NewAgentToolsModel(conn sqlx.SqlConn) AgentToolsModel {
	return &customAgentToolsModel{
		defaultAgentToolsModel: newAgentToolsModel(conn),
	}
}

func (m *customAgentToolsModel) withSession(session sqlx.Session) AgentToolsModel {
	return NewAgentToolsModel(sqlx.NewSqlConnFromSession(session))
}
