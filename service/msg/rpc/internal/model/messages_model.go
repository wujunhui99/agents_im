package model

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ MessagesModel = (*customMessagesModel)(nil)

type (
	// MessagesModel is an interface to be customized, add more methods here,
	// and implement the added methods in customMessagesModel.
	MessagesModel interface {
		messagesModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) MessagesModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排业务事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error

		// FindBySenderClient 按 (sender_account_id, client_msg_id) 幂等键查消息；不存在返回 ErrNotFound。
		FindBySenderClient(ctx context.Context, senderID, clientMsgID string) (*Messages, error)
		// InsertReturning 插入一行 messages（content 走 ::jsonb）并返回插入后的完整行。
		// 调用方需先填好 int 枚举列、jsonb content、seq、payload_hash、client_send_time。
		InsertReturning(ctx context.Context, data *Messages) (*Messages, error)
		// GetMessagesInRange 按 seq 区间分页拉取，返回 (消息, isEnd, nextSeq)。
		GetMessagesInRange(ctx context.Context, conversationID string, fromSeq, toSeq, maxSeq int64, limit int, order string) ([]*Messages, bool, int64, error)
	}

	customMessagesModel struct {
		*defaultMessagesModel
	}
)

// NewMessagesModel returns a model for the database table.
func NewMessagesModel(conn sqlx.SqlConn) MessagesModel {
	return &customMessagesModel{
		defaultMessagesModel: newMessagesModel(conn),
	}
}

func (m *customMessagesModel) WithSession(session sqlx.Session) MessagesModel {
	return NewMessagesModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customMessagesModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customMessagesModel) FindBySenderClient(ctx context.Context, senderID, clientMsgID string) (*Messages, error) {
	query := fmt.Sprintf(`select %s from %s where sender_account_id = $1 and client_msg_id = $2 limit 1`, messagesRows, m.table)
	var resp Messages
	switch err := m.conn.QueryRowCtx(ctx, &resp, query, senderID, clientMsgID); err {
	case nil:
		return &resp, nil
	case sqlx.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

func (m *customMessagesModel) InsertReturning(ctx context.Context, data *Messages) (*Messages, error) {
	query := fmt.Sprintf(`insert into %s (
	  message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
	  receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
	  trigger_message_id, agent_run_id, allow_recursive_trigger, payload_hash, client_send_time
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
returning %s`, m.table, messagesRows)
	var resp Messages
	if err := m.conn.QueryRowCtx(ctx, &resp, query,
		data.MessageId, data.ClientMsgId, data.SenderAccountId, data.ConversationId, data.Seq, data.ConversationType,
		data.ReceiverAccountId, data.GroupId, data.ContentType, data.Content, data.MessageOrigin, data.AgentAccountId,
		data.TriggerMessageId, data.AgentRunId, data.AllowRecursiveTrigger, data.PayloadHash, data.ClientSendTime); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (m *customMessagesModel) GetMessagesInRange(ctx context.Context, conversationID string, fromSeq, toSeq, maxSeq int64, limit int, order string) ([]*Messages, bool, int64, error) {
	if toSeq <= 0 || toSeq > maxSeq {
		toSeq = maxSeq
	}
	if fromSeq > toSeq || maxSeq == 0 {
		return []*Messages{}, true, fromSeq, nil
	}
	if order != OrderAsc && order != OrderDesc {
		order = OrderAsc
	}

	query := fmt.Sprintf(`select %s from %s
where conversation_id = $1 and seq >= $2 and seq <= $3
order by seq %s
limit $4`, messagesRows, m.table, order)
	var rows []*Messages
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, conversationID, fromSeq, toSeq, limit+1); err != nil {
		return nil, false, 0, err
	}

	isEnd := true
	if len(rows) > limit {
		isEnd = false
		rows = rows[:limit]
	}

	nextSeq := fromSeq
	if len(rows) > 0 {
		if order == OrderDesc {
			nextSeq = rows[len(rows)-1].Seq - 1
		} else {
			nextSeq = rows[len(rows)-1].Seq + 1
		}
	}
	return rows, isEnd, nextSeq, nil
}

// MessageSendTime 把 (client_send_time, server_received_at) 折叠成毫秒时间戳，
// 复刻旧实现的 coalesce(client_send_time, server_received_at) 语义。
func MessageSendTime(m *Messages) int64 {
	if m == nil {
		return 0
	}
	if m.ClientSendTime.Valid {
		return m.ClientSendTime.Time.UTC().UnixMilli()
	}
	return m.ServerReceivedAt.UTC().UnixMilli()
}

// DecodeMessageContent 还原存库 jsonb（text 消息存成 {"text":...}）为对外 content 字符串。
func DecodeMessageContent(content string) string {
	var textBody struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &textBody); err == nil && textBody.Text != "" {
		return textBody.Text
	}
	return content
}

// EncodeMessageContent 把入站 content 规范成存库 jsonb（text 包成 {"text":...}，其余透传原 JSON）。
func EncodeMessageContent(contentType, content string) (string, error) {
	if contentType == ContentTypeText {
		encoded, err := json.Marshal(struct {
			Text string `json:"text"`
		}{Text: content})
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return "", err
	}
	return string(raw), nil
}
