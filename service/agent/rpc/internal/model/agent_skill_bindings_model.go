package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var _ AgentSkillBindingsModel = (*customAgentSkillBindingsModel)(nil)

type (
	// AgentSkillBindingsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentSkillBindingsModel.
	AgentSkillBindingsModel interface {
		agentSkillBindingsModel
		withSession(session sqlx.Session) AgentSkillBindingsModel
	}

	customAgentSkillBindingsModel struct {
		*defaultAgentSkillBindingsModel
	}
)

// NewAgentSkillBindingsModel returns a model for the database table.
func NewAgentSkillBindingsModel(conn sqlx.SqlConn) AgentSkillBindingsModel {
	return &customAgentSkillBindingsModel{
		defaultAgentSkillBindingsModel: newAgentSkillBindingsModel(conn),
	}
}

func (m *customAgentSkillBindingsModel) withSession(session sqlx.Session) AgentSkillBindingsModel {
	return NewAgentSkillBindingsModel(sqlx.NewSqlConnFromSession(session))
}
