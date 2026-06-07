package model

import (
	"context"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ConversationThreadsModel = (*customConversationThreadsModel)(nil)

type (
	// ConversationThreadsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customConversationThreadsModel.
	ConversationThreadsModel interface {
		conversationThreadsModel
		// WithSession 返回绑定到给定事务 session 的 model。
		WithSession(session sqlx.Session) ConversationThreadsModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// UpsertAndLock 确保会话行存在（ON CONFLICT DO NOTHING），并 FOR UPDATE 锁定返回当前 max_seq。
		// 必须在事务 session 内调用（提供 seq 分配的串行化保证）。
		UpsertAndLock(ctx context.Context, conversationID string, params UpsertConversationParams) (int64, error)
		// UpdateAfterMessage 在写入消息后推进 max_seq / last_message_id / last_message_at。
		UpdateAfterMessage(ctx context.Context, conversationID, serverMsgID string, seq int64, sendTime time.Time) error
		// GetMaxSeq 读取会话 max_seq；会话不存在返回 ErrNotFound。
		GetMaxSeq(ctx context.Context, conversationID string) (int64, error)
	}

	// UpsertConversationParams 描述会话的不变属性（用于首次创建行）。
	UpsertConversationParams struct {
		ChatType      string
		SingleUserA   string
		SingleUserB   string
		GroupID       string
	}

	customConversationThreadsModel struct {
		*defaultConversationThreadsModel
	}
)

// NewConversationThreadsModel returns a model for the database table.
func NewConversationThreadsModel(conn sqlx.SqlConn) ConversationThreadsModel {
	return &customConversationThreadsModel{
		defaultConversationThreadsModel: newConversationThreadsModel(conn),
	}
}

func (m *customConversationThreadsModel) WithSession(session sqlx.Session) ConversationThreadsModel {
	return NewConversationThreadsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customConversationThreadsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customConversationThreadsModel) UpsertAndLock(ctx context.Context, conversationID string, params UpsertConversationParams) (int64, error) {
	conversationType, err := ConversationTypeValue(params.ChatType)
	if err != nil {
		return 0, err
	}
	switch params.ChatType {
	case ChatTypeSingle:
		userA, userB := orderedSingleUsers(params.SingleUserA, params.SingleUserB)
		if _, err := m.conn.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, conversation_type, single_account_a, single_account_b)
values ($1, $2, $3, $4)
on conflict (conversation_id) do nothing
`, conversationID, conversationType, userA, userB); err != nil {
			return 0, err
		}
	case ChatTypeGroup:
		if _, err := m.conn.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, conversation_type, group_id)
values ($1, $2, $3)
on conflict (conversation_id) do nothing
`, conversationID, conversationType, params.GroupID); err != nil {
			return 0, err
		}
	default:
		return 0, apperror.InvalidArgument("chat_type must be single or group")
	}

	var maxSeq int64
	err = m.conn.QueryRowCtx(ctx, &maxSeq, `
select max_seq
from conversation_threads
where conversation_id = $1
for update
`, conversationID)
	return maxSeq, err
}

func (m *customConversationThreadsModel) UpdateAfterMessage(ctx context.Context, conversationID, serverMsgID string, seq int64, sendTime time.Time) error {
	_, err := m.conn.ExecCtx(ctx, `
update conversation_threads
set max_seq = $2,
    last_message_id = $3,
    last_message_at = $4,
    updated_at = now()
where conversation_id = $1
`, conversationID, seq, serverMsgID, sendTime)
	return err
}

func (m *customConversationThreadsModel) GetMaxSeq(ctx context.Context, conversationID string) (int64, error) {
	var maxSeq int64
	err := m.conn.QueryRowCtx(ctx, &maxSeq, `select max_seq from conversation_threads where conversation_id = $1`, conversationID)
	return maxSeq, err
}
