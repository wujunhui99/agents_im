package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ConversationSeqState 是单个会话的 seq 同步视图（threads + states + 末条消息 的 join 投影）。
type ConversationSeqState struct {
	ConversationID string
	MaxSeq         int64
	HasReadSeq     int64
	UnreadCount    int64
	MaxSeqTime     int64
	LastMessage    *Messages
}

func (s ConversationSeqState) Clone() ConversationSeqState {
	if s.LastMessage != nil {
		last := *s.LastMessage
		s.LastMessage = &last
	}
	return s
}

// conversationStateRow 映射 QuerySeqState 的 join 投影（非单表行）。
type conversationStateRow struct {
	ConversationID  string       `db:"conversation_id"`
	MaxSeq          int64        `db:"max_seq"`
	HasReadSeq      int64        `db:"has_read_seq"`
	VisibleStartSeq int64        `db:"visible_start_seq"`
	MaxSeqTime      sql.NullTime `db:"max_seq_time"`
	LastMessageID   string       `db:"last_message_id"`
	UpdatedAt       time.Time    `db:"updated_at"`
}

var _ UserConversationStatesModel = (*customUserConversationStatesModel)(nil)

type (
	// UserConversationStatesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserConversationStatesModel.
	UserConversationStatesModel interface {
		userConversationStatesModel
		// WithSession 返回绑定到给定事务 session 的 model。
		WithSession(session sqlx.Session) UserConversationStatesModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// UpsertVisible 为可见成员建/补 user_conversation_states 行（首建写 visible_start_seq）。
		UpsertVisible(ctx context.Context, userIDs []string, conversationID string, visibleStartSeq int64) error
		// UpsertSenderRead 把发送者的已读 seq 推进到刚写入的 seq（首建 visible_start=0）。
		UpsertSenderRead(ctx context.Context, senderID, conversationID string, seq int64) error
		// RepairDirect 为单聊补建当前用户的可见状态行（visible_start 归零修复）。
		RepairDirect(ctx context.Context, userID, conversationID string) error
		// RepairAllDirect 为当前用户所有单聊会话补建可见状态行。
		RepairAllDirect(ctx context.Context, userID string) error
		// ListConversationIDs 返回用户有状态行的会话 id（按 updated_at 倒序）。
		ListConversationIDs(ctx context.Context, userID string) ([]string, error)
		// LockReadState FOR UPDATE 读取 (last_read_seq, visible_start_seq, max_seq)；不存在返回 ErrNotFound。
		LockReadState(ctx context.Context, userID, conversationID string) (ReadStateBounds, error)
		// AdvanceLastReadSeq 把 last_read_seq 推进到 greatest(current, seq, visible_start_seq)。
		AdvanceLastReadSeq(ctx context.Context, userID, conversationID string, seq int64) error
		// UserScopedBounds 读取用户视角的 (max_seq, visible_start_seq)；不存在返回 ErrNotFound。
		UserScopedBounds(ctx context.Context, userID, conversationID string) (ReadStateBounds, error)
		// QuerySeqState 返回单个会话的 seq 同步视图（join threads + states + last message）。
		QuerySeqState(ctx context.Context, userID, conversationID string) (ConversationSeqState, error)
	}

	// ReadStateBounds 聚合一次读取里需要的 seq 边界。
	ReadStateBounds struct {
		HasReadSeq      int64
		MaxSeq          int64
		VisibleStartSeq int64
	}

	customUserConversationStatesModel struct {
		*defaultUserConversationStatesModel
	}
)

// NewUserConversationStatesModel returns a model for the database table.
func NewUserConversationStatesModel(conn sqlx.SqlConn) UserConversationStatesModel {
	return &customUserConversationStatesModel{
		defaultUserConversationStatesModel: newUserConversationStatesModel(conn),
	}
}

func (m *customUserConversationStatesModel) WithSession(session sqlx.Session) UserConversationStatesModel {
	return NewUserConversationStatesModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customUserConversationStatesModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customUserConversationStatesModel) UpsertVisible(ctx context.Context, userIDs []string, conversationID string, visibleStartSeq int64) error {
	for _, userID := range userIDs {
		if _, err := m.conn.ExecCtx(ctx, `
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
values ($1, $2, $3, $3)
on conflict (account_id, conversation_id) do update
set updated_at = now()
`, userID, conversationID, visibleStartSeq); err != nil {
			return err
		}
	}
	return nil
}

func (m *customUserConversationStatesModel) UpsertSenderRead(ctx context.Context, senderID, conversationID string, seq int64) error {
	_, err := m.conn.ExecCtx(ctx, `
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
values ($1, $2, $3, 0)
on conflict (account_id, conversation_id) do update
set last_read_seq = greatest(user_conversation_states.last_read_seq, excluded.last_read_seq),
    updated_at = now()
`, senderID, conversationID, seq)
	return err
}

func (m *customUserConversationStatesModel) RepairDirect(ctx context.Context, userID, conversationID string) error {
	_, err := m.conn.ExecCtx(ctx, `
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
select $1, t.conversation_id, 0, 0
from conversation_threads t
where t.conversation_id = $2
  and t.conversation_type = $3
  and (t.single_account_a = $1 or t.single_account_b = $1)
on conflict (account_id, conversation_id) do update
set visible_start_seq = 0,
    updated_at = case
      when user_conversation_states.visible_start_seq <> 0 then now()
      else user_conversation_states.updated_at
    end
where user_conversation_states.visible_start_seq <> 0
`, userID, conversationID, ConversationTypeSingle)
	return err
}

func (m *customUserConversationStatesModel) RepairAllDirect(ctx context.Context, userID string) error {
	_, err := m.conn.ExecCtx(ctx, `
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
select $1, t.conversation_id, 0, 0
from conversation_threads t
where t.conversation_type = $2
  and (t.single_account_a = $1 or t.single_account_b = $1)
on conflict (account_id, conversation_id) do update
set visible_start_seq = 0,
    updated_at = case
      when user_conversation_states.visible_start_seq <> 0 then now()
      else user_conversation_states.updated_at
    end
where user_conversation_states.visible_start_seq <> 0
`, userID, ConversationTypeSingle)
	return err
}

func (m *customUserConversationStatesModel) ListConversationIDs(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	if err := m.conn.QueryRowsCtx(ctx, &ids, `
select conversation_id
from user_conversation_states
where account_id = $1
order by updated_at desc, conversation_id asc
`, userID); err != nil {
		return nil, err
	}
	return ids, nil
}

func (m *customUserConversationStatesModel) LockReadState(ctx context.Context, userID, conversationID string) (ReadStateBounds, error) {
	var row struct {
		HasReadSeq      int64 `db:"last_read_seq"`
		VisibleStartSeq int64 `db:"visible_start_seq"`
		MaxSeq          int64 `db:"max_seq"`
	}
	if err := m.conn.QueryRowCtx(ctx, &row, `
select s.last_read_seq, s.visible_start_seq, t.max_seq
from user_conversation_states s
join conversation_threads t on t.conversation_id = s.conversation_id
where s.account_id = $1 and s.conversation_id = $2
for update
`, userID, conversationID); err != nil {
		return ReadStateBounds{}, err
	}
	return ReadStateBounds{HasReadSeq: row.HasReadSeq, MaxSeq: row.MaxSeq, VisibleStartSeq: row.VisibleStartSeq}, nil
}

func (m *customUserConversationStatesModel) AdvanceLastReadSeq(ctx context.Context, userID, conversationID string, seq int64) error {
	_, err := m.conn.ExecCtx(ctx, `
update user_conversation_states
set last_read_seq = greatest(last_read_seq, $3, visible_start_seq),
    updated_at = now()
where account_id = $1 and conversation_id = $2
`, userID, conversationID, seq)
	return err
}

func (m *customUserConversationStatesModel) UserScopedBounds(ctx context.Context, userID, conversationID string) (ReadStateBounds, error) {
	var row struct {
		MaxSeq          int64 `db:"max_seq"`
		VisibleStartSeq int64 `db:"visible_start_seq"`
	}
	if err := m.conn.QueryRowCtx(ctx, &row, `
select t.max_seq, s.visible_start_seq
from conversation_threads t
join user_conversation_states s on s.conversation_id = t.conversation_id
where s.account_id = $1 and t.conversation_id = $2
`, userID, conversationID); err != nil {
		return ReadStateBounds{}, err
	}
	return ReadStateBounds{MaxSeq: row.MaxSeq, VisibleStartSeq: row.VisibleStartSeq}, nil
}

func (m *customUserConversationStatesModel) QuerySeqState(ctx context.Context, userID, conversationID string) (ConversationSeqState, error) {
	var row conversationStateRow
	if err := m.conn.QueryRowCtx(ctx, &row, `
select t.conversation_id,
       t.max_seq,
       s.last_read_seq as has_read_seq,
       s.visible_start_seq,
       lm.client_send_time as max_seq_time,
       case when t.max_seq > s.visible_start_seq then coalesce(t.last_message_id, '') else '' end as last_message_id,
       s.updated_at
from conversation_threads t
join user_conversation_states s on s.conversation_id = t.conversation_id
left join messages lm on lm.message_id::text = t.last_message_id
where s.account_id = $1 and t.conversation_id = $2
`, userID, conversationID); err != nil {
		return ConversationSeqState{}, err
	}

	state := ConversationSeqState{
		ConversationID: row.ConversationID,
		MaxSeq:         row.MaxSeq,
		HasReadSeq:     row.HasReadSeq,
		UnreadCount:    unreadCountFromVisibleStart(row.MaxSeq, row.HasReadSeq, row.VisibleStartSeq),
	}
	if row.MaxSeqTime.Valid {
		state.MaxSeqTime = row.MaxSeqTime.Time.UTC().UnixMilli()
	}
	if row.LastMessageID != "" {
		last, err := m.fetchMessageByID(ctx, row.LastMessageID)
		if err != nil {
			return ConversationSeqState{}, err
		}
		state.LastMessage = last
	}
	return state, nil
}

func (m *customUserConversationStatesModel) fetchMessageByID(ctx context.Context, serverMsgID string) (*Messages, error) {
	query := fmt.Sprintf(`select %s from messages where message_id = $1 limit 1`, messagesRows)
	var resp Messages
	if err := m.conn.QueryRowCtx(ctx, &resp, query, serverMsgID); err != nil {
		return nil, err
	}
	return &resp, nil
}
