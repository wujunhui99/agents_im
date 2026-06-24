package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentSkillBindingsModel = (*customAgentSkillBindingsModel)(nil)

type (
	// AgentSkillBindingsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentSkillBindingsModel.
	AgentSkillBindingsModel interface {
		agentSkillBindingsModel

		// BindOne 幂等绑定（已存在返回 created=false）。
		BindOne(ctx context.Context, agentID, skillID int64, createdBy string) (*AgentSkillBindings, bool, error)
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
