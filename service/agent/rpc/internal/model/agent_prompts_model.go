package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AgentPromptsModel = (*customAgentPromptsModel)(nil)

type (
	// AgentPromptsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentPromptsModel.
	AgentPromptsModel interface {
		agentPromptsModel
		withSession(session sqlx.Session) AgentPromptsModel
	}

	customAgentPromptsModel struct {
		*defaultAgentPromptsModel
	}
)

// NewAgentPromptsModel returns a model for the database table.
func NewAgentPromptsModel(conn sqlx.SqlConn) AgentPromptsModel {
	return &customAgentPromptsModel{
		defaultAgentPromptsModel: newAgentPromptsModel(conn),
	}
}

func (m *customAgentPromptsModel) withSession(session sqlx.Session) AgentPromptsModel {
	return NewAgentPromptsModel(sqlx.NewSqlConnFromSession(session))
}
