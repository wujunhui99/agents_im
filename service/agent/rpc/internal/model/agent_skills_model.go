package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentSkillsModel = (*customAgentSkillsModel)(nil)

type (
	// AgentSkillsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentSkillsModel.
	AgentSkillsModel interface {
		agentSkillsModel
		withSession(session sqlx.Session) AgentSkillsModel

		// InsertReturning 插入并返回库生成的完整行（skill_id 自增）。
		InsertReturning(ctx context.Context, data *AgentSkills) (*AgentSkills, error)
	}

	customAgentSkillsModel struct {
		*defaultAgentSkillsModel
	}
)

// NewAgentSkillsModel returns a model for the database table.
func NewAgentSkillsModel(conn sqlx.SqlConn) AgentSkillsModel {
	return &customAgentSkillsModel{
		defaultAgentSkillsModel: newAgentSkillsModel(conn),
	}
}

func (m *customAgentSkillsModel) withSession(session sqlx.Session) AgentSkillsModel {
	return NewAgentSkillsModel(sqlx.NewSqlConnFromSession(session))
}
