package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentToolsModel = (*customAgentToolsModel)(nil)

type (
	// AgentToolsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentToolsModel.
	AgentToolsModel interface {
		agentToolsModel

		// InsertReturning 插入并返回库生成的完整行（tool_id 自增、jsonb schema 列）。
		InsertReturning(ctx context.Context, data *AgentTools) (*AgentTools, error)
		// UpsertByName 按 name 唯一键 upsert（DefaultAssistant 幂等装配工具），返回结果行。
		UpsertByName(ctx context.Context, data *AgentTools) (*AgentTools, error)
		// ListActive 列出 status='active' 的工具，按 name 升序（agent.create 默认可绑工具集来源）。
		ListActive(ctx context.Context) ([]*AgentTools, error)
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
