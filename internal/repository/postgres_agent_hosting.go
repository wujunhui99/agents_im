package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type postgresAgentConversationHostingRow struct {
	ConversationID             string    `db:"conversation_id"`
	AgentAccountID             string    `db:"agent_account_id"`
	Enabled                    bool      `db:"enabled"`
	AllowAgentMessageRecursion bool      `db:"allow_agent_message_recursion"`
	CreatedAt                  time.Time `db:"created_at"`
	UpdatedAt                  time.Time `db:"updated_at"`
}

func NewPostgresAgentConversationHostingRepository(dataSource string) (*PostgresRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresRepositoryFromConn(postgres.New(dataSource)), nil
}

func (r *PostgresRepository) UpsertAgentConversationHosting(ctx context.Context, hosting AgentConversationHosting) (AgentConversationHosting, error) {
	hosting, err := normalizeAgentConversationHosting(hosting)
	if err != nil {
		return AgentConversationHosting{}, err
	}

	var row postgresAgentConversationHostingRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_conversation_hosting (
  conversation_id, agent_account_id, enabled, allow_agent_message_recursion
)
values ($1, $2, $3, $4)
on conflict (conversation_id) do update
set agent_account_id = excluded.agent_account_id,
    enabled = excluded.enabled,
    allow_agent_message_recursion = excluded.allow_agent_message_recursion,
    updated_at = now()
returning conversation_id, agent_account_id, enabled, allow_agent_message_recursion, created_at, updated_at
`, hosting.ConversationID, hosting.AgentAccountID, hosting.Enabled, hosting.AllowAgentMessageRecursion)
	if err != nil {
		if isPostgresCheckViolation(err) {
			return AgentConversationHosting{}, apperror.InvalidArgument("invalid agent conversation hosting")
		}
		return AgentConversationHosting{}, err
	}
	return row.agentConversationHosting().Clone(), nil
}

func (r *PostgresRepository) GetAgentConversationHosting(ctx context.Context, conversationID string) (AgentConversationHosting, error) {
	conversationID, err := normalizeAgentHostingRequired(conversationID, "conversation_id")
	if err != nil {
		return AgentConversationHosting{}, err
	}

	var row postgresAgentConversationHostingRow
	if err := r.conn.QueryRowCtx(ctx, &row, `
select conversation_id, agent_account_id, enabled, allow_agent_message_recursion, created_at, updated_at
from agent_conversation_hosting
where conversation_id = $1
`, conversationID); err != nil {
		if isNotFound(err) {
			return AgentConversationHosting{}, apperror.NotFound("agent conversation hosting not found")
		}
		return AgentConversationHosting{}, err
	}
	return row.agentConversationHosting().Clone(), nil
}

func (r *PostgresRepository) TryStartAgentTrigger(ctx context.Context, input AgentTriggerStartInput) (bool, error) {
	input, err := normalizeAgentTriggerStartInput(input)
	if err != nil {
		return false, err
	}
	runningTTLMillis := input.RunningTTL.Milliseconds()
	if runningTTLMillis < 1 {
		runningTTLMillis = 1
	}

	var started bool
	err = r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, `
insert into agent_trigger_idempotency (
  idempotency_key, conversation_id, agent_account_id, trigger_server_msg_id, trigger_event_id, status
)
values ($1, $2, $3, $4, $5, $6)
on conflict (idempotency_key) do nothing
`, input.IdempotencyKey, input.ConversationID, input.AgentAccountID, input.TriggerServerMsgID, input.TriggerEventID, AgentTriggerStatusRunning)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows > 0 {
			started = true
			return nil
		}

		result, err = session.ExecCtx(ctx, `
update agent_trigger_idempotency
set conversation_id = $2,
    agent_account_id = $3,
    trigger_server_msg_id = $4,
    trigger_event_id = $5,
    status = $6,
    response_server_msg_id = '',
    error_message = '',
    updated_at = now()
where idempotency_key = $1
  and (
    status = $7
    or (status = $8 and updated_at <= now() - ($9 * interval '1 millisecond'))
  )
`, input.IdempotencyKey, input.ConversationID, input.AgentAccountID, input.TriggerServerMsgID,
			input.TriggerEventID, AgentTriggerStatusRunning, AgentTriggerStatusFailed, AgentTriggerStatusRunning, runningTTLMillis)
		if err != nil {
			return err
		}
		rows, err = result.RowsAffected()
		if err != nil {
			return err
		}
		started = rows > 0
		return nil
	})
	if err != nil {
		if isPostgresCheckViolation(err) {
			return false, apperror.InvalidArgument("invalid agent trigger idempotency input")
		}
		return false, err
	}
	return started, nil
}

func (r *PostgresRepository) FinishAgentTrigger(ctx context.Context, input AgentTriggerFinishInput) error {
	input, err := normalizeAgentTriggerFinishInput(input)
	if err != nil {
		return err
	}

	result, err := r.conn.ExecCtx(ctx, `
update agent_trigger_idempotency
set status = $2,
    response_server_msg_id = $3,
    error_message = $4,
    updated_at = now()
where idempotency_key = $1 and status = $5
`, input.IdempotencyKey, input.Status, input.ResponseServerMsgID, input.ErrorMessage, AgentTriggerStatusRunning)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperror.NotFound("agent trigger idempotency key not found")
	}
	return nil
}

func (r postgresAgentConversationHostingRow) agentConversationHosting() AgentConversationHosting {
	return AgentConversationHosting{
		ConversationID:             r.ConversationID,
		AgentAccountID:             r.AgentAccountID,
		Enabled:                    r.Enabled,
		AllowAgentMessageRecursion: r.AllowAgentMessageRecursion,
		CreatedAt:                  r.CreatedAt.UTC(),
		UpdatedAt:                  r.UpdatedAt.UTC(),
	}
}
