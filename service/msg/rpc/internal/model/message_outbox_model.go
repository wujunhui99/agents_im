package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MessageOutboxModel = (*customMessageOutboxModel)(nil)

type (
	// MessageOutboxModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMessageOutboxModel.
	MessageOutboxModel interface {
		messageOutboxModel
		// WithSession 返回绑定到给定事务 session 的 model。
		WithSession(session sqlx.Session) MessageOutboxModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// InsertCreatedEvent 写入一条 message.created outbox 事件（payload 由 Logic 层构造好）。
		// payload JSON 必须与 msgtransfer 消费方 (internal/outboxpublisher) 约定一致。
		InsertCreatedEvent(ctx context.Context, params OutboxCreatedEventParams) error
	}

	// OutboxCreatedEventParams 描述一条 message.created 事件的列值。
	OutboxCreatedEventParams struct {
		EventID        string
		ConversationID string
		MessageID      string
		Seq            int64
		Payload        string
	}

	customMessageOutboxModel struct {
		*defaultMessageOutboxModel
	}
)

// NewMessageOutboxModel returns a model for the database table.
func NewMessageOutboxModel(conn sqlx.SqlConn) MessageOutboxModel {
	return &customMessageOutboxModel{
		defaultMessageOutboxModel: newMessageOutboxModel(conn),
	}
}

func (m *customMessageOutboxModel) WithSession(session sqlx.Session) MessageOutboxModel {
	return NewMessageOutboxModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customMessageOutboxModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customMessageOutboxModel) InsertCreatedEvent(ctx context.Context, params OutboxCreatedEventParams) error {
	_, err := m.conn.ExecCtx(ctx, `
insert into message_outbox (
	  event_id, event_type, aggregate_type, aggregate_id, conversation_id, message_id,
	  seq, payload, status, next_attempt_at
)
values ($1, $2::smallint, $3::smallint, $4, $5, $6, $7, $8::jsonb, $9::smallint, now())
`, params.EventID, OutboxEventTypeMessageCreated, OutboxAggregateTypeMessage, params.MessageID,
		params.ConversationID, params.MessageID, params.Seq, params.Payload, OutboxStatusPending)
	return err
}
