package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type postgresOutboxRow struct {
	EventID        string       `db:"event_id"`
	EventType      string       `db:"event_type"`
	AggregateType  string       `db:"aggregate_type"`
	AggregateID    string       `db:"aggregate_id"`
	ConversationID string       `db:"conversation_id"`
	ServerMsgID    string       `db:"server_msg_id"`
	Seq            int64        `db:"seq"`
	Payload        []byte       `db:"payload"`
	Status         string       `db:"status"`
	AttemptCount   int          `db:"attempt_count"`
	NextAttemptAt  time.Time    `db:"next_attempt_at"`
	LockedBy       string       `db:"locked_by"`
	LockedUntil    sql.NullTime `db:"locked_until"`
	LastError      string       `db:"last_error"`
	CreatedAt      time.Time    `db:"created_at"`
	UpdatedAt      time.Time    `db:"updated_at"`
	PublishedAt    sql.NullTime `db:"published_at"`
}

func (r *PostgresMessageRepository) PollPending(ctx context.Context, workerID string, limit int, lockDuration time.Duration) ([]OutboxEvent, error) {
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, apperror.InvalidArgument("worker_id is required")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if lockDuration <= 0 {
		lockDuration = 30 * time.Second
	}
	lockedUntil := r.now().UTC().Add(lockDuration)

	var rows []postgresOutboxRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
with picked as (
  select event_id
  from message_outbox
  where status = $1
    and next_attempt_at <= now()
    and (locked_until is null or locked_until <= now())
  order by created_at asc, event_id asc
  limit $2
  FOR UPDATE SKIP LOCKED
)
update message_outbox o
set locked_by = $3,
    locked_until = $4,
    updated_at = now()
from picked
where o.event_id = picked.event_id
returning o.event_id, o.event_type, o.aggregate_type, o.aggregate_id,
          o.conversation_id, o.server_msg_id, o.seq, o.payload, o.status,
          o.attempt_count, o.next_attempt_at, o.locked_by, o.locked_until,
          o.last_error, o.created_at, o.updated_at, o.published_at
`, OutboxStatusPending, limit, workerID, lockedUntil); err != nil {
		return nil, err
	}

	events := make([]OutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, row.outboxEvent().Clone())
	}
	return events, nil
}

func (r *PostgresMessageRepository) MarkPublished(ctx context.Context, eventID string, workerID string) error {
	eventID = strings.TrimSpace(eventID)
	workerID = strings.TrimSpace(workerID)
	if eventID == "" {
		return apperror.InvalidArgument("event_id is required")
	}
	if workerID == "" {
		return apperror.InvalidArgument("worker_id is required")
	}

	return r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, `
update message_outbox
set status = $3,
    locked_by = '',
    locked_until = null,
    published_at = now(),
    updated_at = now()
where event_id = $1
  and locked_by = $2
  and status = $4
  and locked_until > now()
`, eventID, workerID, OutboxStatusPublished, OutboxStatusPending)
		if err != nil {
			return err
		}
		if err := requireOutboxRowAffected(result); err != nil {
			return err
		}
		return markDeliveryAttemptsPublishedForOutboxEvent(ctx, session, eventID)
	})
}

func (r *PostgresMessageRepository) MarkFailed(ctx context.Context, eventID string, workerID string, failure OutboxFailure) error {
	eventID = strings.TrimSpace(eventID)
	workerID = strings.TrimSpace(workerID)
	if eventID == "" {
		return apperror.InvalidArgument("event_id is required")
	}
	if workerID == "" {
		return apperror.InvalidArgument("worker_id is required")
	}

	status := OutboxStatusFailed
	nextAttemptAt := r.now().UTC()
	if !failure.NextAttemptAt.IsZero() {
		status = OutboxStatusPending
		nextAttemptAt = failure.NextAttemptAt.UTC()
	}

	result, err := r.conn.ExecCtx(ctx, `
update message_outbox
set status = $3,
    attempt_count = attempt_count + 1,
    next_attempt_at = $4,
    locked_by = '',
    locked_until = null,
    last_error = $5,
    updated_at = now()
where event_id = $1
  and locked_by = $2
  and status = $6
  and locked_until > now()
`, eventID, workerID, status, nextAttemptAt, strings.TrimSpace(failure.LastError), OutboxStatusPending)
	if err != nil {
		return err
	}
	return requireOutboxRowAffected(result)
}

func insertMessageOutboxEvent(ctx context.Context, session sqlx.Session, message postgresMessageRow, input CreateMessageInput) error {
	payload, err := messageCreatedOutboxPayload(message.message(), input)
	if err != nil {
		return err
	}
	_, err = session.ExecCtx(ctx, `
insert into message_outbox (
  event_type, aggregate_type, aggregate_id, conversation_id, server_msg_id,
  seq, payload, status, next_attempt_at
)
values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, now())
`, OutboxEventTypeMessageCreated, OutboxAggregateTypeMessage, message.ServerMsgID,
		message.ConversationID, message.ServerMsgID, message.Seq, string(payload), OutboxStatusPending)
	return err
}

func requireOutboxRowAffected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return apperror.NotFound("outbox event lock not found")
	}
	return nil
}

func (r postgresOutboxRow) outboxEvent() OutboxEvent {
	event := OutboxEvent{
		EventID:        r.EventID,
		EventType:      r.EventType,
		AggregateType:  r.AggregateType,
		AggregateID:    r.AggregateID,
		ConversationID: r.ConversationID,
		ServerMsgID:    r.ServerMsgID,
		Seq:            r.Seq,
		Payload:        append(json.RawMessage(nil), r.Payload...),
		Status:         r.Status,
		AttemptCount:   r.AttemptCount,
		NextAttemptAt:  r.NextAttemptAt.UTC(),
		LockedBy:       r.LockedBy,
		LastError:      r.LastError,
		CreatedAt:      r.CreatedAt.UTC(),
		UpdatedAt:      r.UpdatedAt.UTC(),
	}
	if r.LockedUntil.Valid {
		event.LockedUntil = r.LockedUntil.Time.UTC()
	}
	if r.PublishedAt.Valid {
		event.PublishedAt = r.PublishedAt.Time.UTC()
	}
	return event
}
