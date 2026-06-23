package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ConversationAiHostingSettingsModel = (*customConversationAiHostingSettingsModel)(nil)

type (
	// ConversationAiHostingSettingsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customConversationAiHostingSettingsModel.
	ConversationAiHostingSettingsModel interface {
		conversationAiHostingSettingsModel
		withSession(session sqlx.Session) ConversationAiHostingSettingsModel

		// FindEnabledByConversationId 返回会话内当前已开启的托管行（partial unique index 保证至多一行）。
		// 无开启行返回 ErrNotFound。
		FindEnabledByConversationId(ctx context.Context, conversationId string) (*ConversationAiHostingSettings, error)
		// Upsert 按 (owner_account_id, conversation_id) 唯一约束写入/更新托管开关，返回写入后的行。
		// 对端已开启时由 partial unique index 触发唯一冲突，交由 Logic 层翻译为业务错误。
		Upsert(ctx context.Context, data *ConversationAiHostingSettings) (*ConversationAiHostingSettings, error)
	}

	customConversationAiHostingSettingsModel struct {
		*defaultConversationAiHostingSettingsModel
	}
)

// NewConversationAiHostingSettingsModel returns a model for the database table.
func NewConversationAiHostingSettingsModel(conn sqlx.SqlConn) ConversationAiHostingSettingsModel {
	return &customConversationAiHostingSettingsModel{
		defaultConversationAiHostingSettingsModel: newConversationAiHostingSettingsModel(conn),
	}
}

func (m *customConversationAiHostingSettingsModel) withSession(session sqlx.Session) ConversationAiHostingSettingsModel {
	return NewConversationAiHostingSettingsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customConversationAiHostingSettingsModel) FindEnabledByConversationId(ctx context.Context, conversationId string) (*ConversationAiHostingSettings, error) {
	var resp ConversationAiHostingSettings
	query := "select " + conversationAiHostingSettingsRows + " from " + m.table + " where conversation_id = $1 and enabled = true limit 1"
	err := m.conn.QueryRowCtx(ctx, &resp, query, conversationId)
	switch err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customConversationAiHostingSettingsModel) Upsert(ctx context.Context, data *ConversationAiHostingSettings) (*ConversationAiHostingSettings, error) {
	var resp ConversationAiHostingSettings
	query := "insert into " + m.table + ` (owner_account_id, conversation_id, enabled, mode, max_recent_messages, summary_enabled)
values ($1, $2, $3, $4, $5, $6)
on conflict (owner_account_id, conversation_id) do update
set enabled = excluded.enabled,
    mode = excluded.mode,
    max_recent_messages = excluded.max_recent_messages,
    summary_enabled = excluded.summary_enabled,
    updated_at = now()
returning ` + conversationAiHostingSettingsRows
	err := m.conn.QueryRowCtx(ctx, &resp, query,
		data.OwnerAccountId, data.ConversationId, data.Enabled, data.Mode, data.MaxRecentMessages, data.SummaryEnabled)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
