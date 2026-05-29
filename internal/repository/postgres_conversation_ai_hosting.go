package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/postgres"
)

type postgresConversationAIHostingRow struct {
	OwnerAccountID    string    `db:"owner_account_id"`
	ConversationID    string    `db:"conversation_id"`
	Enabled           bool      `db:"enabled"`
	Mode              string    `db:"mode"`
	MaxRecentMessages int       `db:"max_recent_messages"`
	SummaryEnabled    bool      `db:"summary_enabled"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

func NewPostgresConversationAIHostingRepository(dataSource string) (*PostgresRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresRepositoryFromConn(postgres.New(dataSource)), nil
}

func (r *PostgresRepository) GetConversationAIHostingSetting(ctx context.Context, ownerAccountID string, conversationID string) (ConversationAIHostingSetting, error) {
	ownerAccountID, conversationID, err := normalizeConversationAIHostingOwnerAndConversation(ownerAccountID, conversationID)
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}

	var row postgresConversationAIHostingRow
	if err := r.conn.QueryRowCtx(ctx, &row, `
select owner_account_id, conversation_id, enabled, mode, max_recent_messages, summary_enabled, created_at, updated_at
from conversation_ai_hosting_settings
where owner_account_id = $1 and conversation_id = $2
`, ownerAccountID, conversationID); err != nil {
		if isNotFound(err) {
			return ConversationAIHostingSetting{}, apperror.NotFound("conversation AI hosting setting not found")
		}
		return ConversationAIHostingSetting{}, err
	}
	return row.conversationAIHostingSetting().Clone(), nil
}

func (r *PostgresRepository) GetEnabledConversationAIHosting(ctx context.Context, conversationID string) (ConversationAIHostingSetting, error) {
	conversationID, err := normalizeAgentHostingRequired(conversationID, "conversation_id")
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}

	var row postgresConversationAIHostingRow
	if err := r.conn.QueryRowCtx(ctx, &row, `
select owner_account_id, conversation_id, enabled, mode, max_recent_messages, summary_enabled, created_at, updated_at
from conversation_ai_hosting_settings
where conversation_id = $1 and enabled = true
`, conversationID); err != nil {
		if isNotFound(err) {
			return ConversationAIHostingSetting{}, apperror.NotFound("enabled conversation AI hosting setting not found")
		}
		return ConversationAIHostingSetting{}, err
	}
	return row.conversationAIHostingSetting().Clone(), nil
}

func (r *PostgresRepository) SetConversationAIHostingEnabled(ctx context.Context, input ConversationAIHostingUpdate) (ConversationAIHostingSetting, error) {
	input, err := normalizeConversationAIHostingUpdate(input)
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}

	var row postgresConversationAIHostingRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into conversation_ai_hosting_settings (
  owner_account_id, conversation_id, enabled, mode, max_recent_messages, summary_enabled
)
values ($1, $2, $3, $4, $5, $6)
on conflict (owner_account_id, conversation_id) do update
set enabled = excluded.enabled,
    mode = excluded.mode,
    max_recent_messages = excluded.max_recent_messages,
    summary_enabled = excluded.summary_enabled,
    updated_at = now()
returning owner_account_id, conversation_id, enabled, mode, max_recent_messages, summary_enabled, created_at, updated_at
`, input.OwnerAccountID, input.ConversationID, input.Enabled, ConversationAIHostingModeAutoReply,
		input.MaxRecentMessages, input.SummaryEnabled)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return ConversationAIHostingSetting{}, conversationAIHostingConflictError()
		}
		if isPostgresCheckViolation(err) {
			return ConversationAIHostingSetting{}, apperror.InvalidArgument("invalid conversation AI hosting setting")
		}
		return ConversationAIHostingSetting{}, err
	}
	return row.conversationAIHostingSetting().Clone(), nil
}

func (r postgresConversationAIHostingRow) conversationAIHostingSetting() ConversationAIHostingSetting {
	return ConversationAIHostingSetting{
		OwnerAccountID:    r.OwnerAccountID,
		ConversationID:    r.ConversationID,
		Enabled:           r.Enabled,
		Mode:              r.Mode,
		MaxRecentMessages: r.MaxRecentMessages,
		SummaryEnabled:    r.SummaryEnabled,
		CreatedAt:         r.CreatedAt.UTC(),
		UpdatedAt:         r.UpdatedAt.UTC(),
	}
}
