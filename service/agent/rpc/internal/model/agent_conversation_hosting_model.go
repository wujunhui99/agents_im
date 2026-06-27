package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentConversationHostingModel = (*customAgentConversationHostingModel)(nil)

type (
	// AgentConversationHostingModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentConversationHostingModel.
	AgentConversationHostingModel interface {
		agentConversationHostingModel
		withSession(session sqlx.Session) AgentConversationHostingModel

		// Upsert 按 conversation_id 主键写入/更新托管行（ON CONFLICT do update），返回写入后的行。
		Upsert(ctx context.Context, data *AgentConversationHosting) (*AgentConversationHosting, error)
	}

	customAgentConversationHostingModel struct {
		*defaultAgentConversationHostingModel
	}
)

// NewAgentConversationHostingModel returns a model for the database table.
func NewAgentConversationHostingModel(conn sqlx.SqlConn) AgentConversationHostingModel {
	return &customAgentConversationHostingModel{
		defaultAgentConversationHostingModel: newAgentConversationHostingModel(conn),
	}
}

func (m *customAgentConversationHostingModel) withSession(session sqlx.Session) AgentConversationHostingModel {
	return NewAgentConversationHostingModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentConversationHostingModel) Upsert(ctx context.Context, data *AgentConversationHosting) (*AgentConversationHosting, error) {
	var resp AgentConversationHosting
	query := "insert into " + m.table + ` (conversation_id, agent_account_id, enabled, allow_agent_message_recursion)
values ($1, $2, $3, $4)
on conflict (conversation_id) do update
set agent_account_id = excluded.agent_account_id,
    enabled = excluded.enabled,
    allow_agent_message_recursion = excluded.allow_agent_message_recursion,
    updated_at = now()
returning ` + agentConversationHostingRows
	err := m.conn.QueryRowCtx(ctx, &resp, query,
		data.ConversationId, data.AgentAccountId, data.Enabled, data.AllowAgentMessageRecursion)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
