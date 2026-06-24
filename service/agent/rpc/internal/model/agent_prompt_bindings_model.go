package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentPromptBindingsModel = (*customAgentPromptBindingsModel)(nil)

type (
	// AgentPromptBindingsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentPromptBindingsModel.
	AgentPromptBindingsModel interface {
		agentPromptBindingsModel

		// FindByAgentId 列出某 agent 的全部 prompt 绑定，按 created_at desc, prompt_id 稳定排序。
		FindByAgentId(ctx context.Context, agentId int64) ([]*AgentPromptBindings, error)
		// BindOne 幂等绑定（已存在返回 created=false），用于 DefaultAssistant 装配。
		BindOne(ctx context.Context, agentID, promptID int64, createdBy string) (*AgentPromptBindings, bool, error)
		// ReplaceForAgent 原子替换某 agent 的全部 prompt 绑定（delete+insert 同事务，去重）。
		ReplaceForAgent(ctx context.Context, agentID int64, promptIDs []int64, createdBy string) ([]*AgentPromptBindings, error)
	}

	customAgentPromptBindingsModel struct {
		*defaultAgentPromptBindingsModel
	}
)

func (m *customAgentPromptBindingsModel) FindByAgentId(ctx context.Context, agentId int64) ([]*AgentPromptBindings, error) {
	query := fmt.Sprintf("select %s from %s where agent_id = $1 order by created_at desc, prompt_id", agentPromptBindingsRows, m.table)
	var resp []*AgentPromptBindings
	if err := m.conn.QueryRowsCtx(ctx, &resp, query, agentId); err != nil {
		return nil, err
	}
	return resp, nil
}

// NewAgentPromptBindingsModel returns a model for the database table.
func NewAgentPromptBindingsModel(conn sqlx.SqlConn) AgentPromptBindingsModel {
	return &customAgentPromptBindingsModel{
		defaultAgentPromptBindingsModel: newAgentPromptBindingsModel(conn),
	}
}
