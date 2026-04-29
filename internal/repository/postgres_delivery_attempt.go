package repository

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ DeliveryAttemptRepository = (*PostgresMessageRepository)(nil)

type postgresDeliveryAttemptRow struct {
	ServerMsgID     string       `db:"server_msg_id"`
	ConversationID  string       `db:"conversation_id"`
	RecipientUserID string       `db:"recipient_user_id"`
	Status          string       `db:"status"`
	AttemptCount    int          `db:"attempt_count"`
	LastError       string       `db:"last_error"`
	NextRetryAt     sql.NullTime `db:"next_retry_at"`
	CreatedAt       time.Time    `db:"created_at"`
	UpdatedAt       time.Time    `db:"updated_at"`
}

func (r *PostgresMessageRepository) CreateDeliveryAttemptsAccepted(ctx context.Context, attempts []CreateDeliveryAttemptInput) error {
	return r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return insertDeliveryAttemptsAccepted(ctx, session, attempts)
	})
}

func (r *PostgresMessageRepository) MarkDeliveryAttemptsPublished(ctx context.Context, serverMsgID string, recipientUserIDs []string) error {
	serverMsgID = strings.TrimSpace(serverMsgID)
	if serverMsgID == "" {
		return nil
	}

	return r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		return markDeliveryAttemptsPublished(ctx, session, serverMsgID, recipientUserIDs)
	})
}

func (r *PostgresMessageRepository) RecordDeliveryAttemptResult(ctx context.Context, input RecordDeliveryAttemptInput) error {
	normalized, err := normalizeRecordDeliveryAttemptInput(input)
	if err != nil {
		return err
	}

	nextRetryAt := sql.NullTime{}
	if !normalized.NextRetryAt.IsZero() {
		nextRetryAt = sql.NullTime{Time: normalized.NextRetryAt.UTC(), Valid: true}
	}

	_, err = r.conn.ExecCtx(ctx, `
insert into delivery_attempts (
  server_msg_id, conversation_id, recipient_user_id, status,
  attempt_count, last_error, next_retry_at
)
values ($1, $2, $3, $4, $5, $6, $7)
on conflict (server_msg_id, recipient_user_id) do update
set conversation_id = excluded.conversation_id,
    status = excluded.status,
    attempt_count = greatest(delivery_attempts.attempt_count + 1, excluded.attempt_count),
    last_error = excluded.last_error,
    next_retry_at = excluded.next_retry_at,
    updated_at = now()
`, normalized.ServerMsgID, normalized.ConversationID, normalized.RecipientUserID, normalized.Status,
		normalized.AttemptCount, normalized.LastError, nextRetryAt)
	return err
}

func (r *PostgresMessageRepository) ListDeliveryAttemptsByMessage(ctx context.Context, serverMsgID string) ([]DeliveryAttempt, error) {
	serverMsgID = strings.TrimSpace(serverMsgID)
	if serverMsgID == "" {
		return nil, nil
	}

	var rows []postgresDeliveryAttemptRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
select server_msg_id, conversation_id, recipient_user_id, status, attempt_count,
       last_error, next_retry_at, created_at, updated_at
from delivery_attempts
where server_msg_id = $1
order by recipient_user_id asc
`, serverMsgID); err != nil {
		return nil, err
	}

	attempts := make([]DeliveryAttempt, 0, len(rows))
	for _, row := range rows {
		attempts = append(attempts, row.deliveryAttempt().Clone())
	}
	return attempts, nil
}

func insertDeliveryAttemptsAccepted(ctx context.Context, session sqlx.Session, attempts []CreateDeliveryAttemptInput) error {
	for _, attemptInput := range attempts {
		normalized, err := normalizeCreateDeliveryAttemptInput(attemptInput)
		if err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx, `
insert into delivery_attempts (
  server_msg_id, conversation_id, recipient_user_id, status
)
values ($1, $2, $3, $4)
on conflict (server_msg_id, recipient_user_id) do nothing
`, normalized.ServerMsgID, normalized.ConversationID, normalized.RecipientUserID, DeliveryStatusAccepted); err != nil {
			return err
		}
	}
	return nil
}

func markDeliveryAttemptsPublished(ctx context.Context, session sqlx.Session, serverMsgID string, recipientUserIDs []string) error {
	recipients := deliveryRecipientSet(recipientUserIDs)
	if len(recipients) == 0 {
		_, err := session.ExecCtx(ctx, `
update delivery_attempts
set status = $2,
    updated_at = now()
where server_msg_id = $1
  and status in ($3, $2)
`, serverMsgID, DeliveryStatusPublished, DeliveryStatusAccepted)
		return err
	}

	for recipientUserID := range recipients {
		if _, err := session.ExecCtx(ctx, `
update delivery_attempts
set status = $3,
    updated_at = now()
where server_msg_id = $1
  and recipient_user_id = $2
  and status in ($4, $3)
`, serverMsgID, recipientUserID, DeliveryStatusPublished, DeliveryStatusAccepted); err != nil {
			return err
		}
	}
	return nil
}

func insertDeliveryAttemptsForMessage(ctx context.Context, session sqlx.Session, message postgresMessageRow, input CreateMessageInput) error {
	return insertDeliveryAttemptsAccepted(ctx, session, deliveryAttemptsForMessage(message.message(), input))
}

func markDeliveryAttemptsPublishedForOutboxEvent(ctx context.Context, session sqlx.Session, eventID string) error {
	_, err := session.ExecCtx(ctx, `
update delivery_attempts d
set status = $2,
    updated_at = now()
from message_outbox o
where o.event_id = $1
  and d.server_msg_id = o.server_msg_id
  and d.status in ($3, $2)
`, eventID, DeliveryStatusPublished, DeliveryStatusAccepted)
	return err
}

func (r postgresDeliveryAttemptRow) deliveryAttempt() DeliveryAttempt {
	attempt := DeliveryAttempt{
		ServerMsgID:     r.ServerMsgID,
		ConversationID:  r.ConversationID,
		RecipientUserID: r.RecipientUserID,
		Status:          r.Status,
		AttemptCount:    r.AttemptCount,
		LastError:       r.LastError,
		CreatedAt:       r.CreatedAt.UTC(),
		UpdatedAt:       r.UpdatedAt.UTC(),
	}
	if r.NextRetryAt.Valid {
		attempt.NextRetryAt = r.NextRetryAt.Time.UTC()
	}
	return attempt
}
