package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresMessageRepository struct {
	conn sqlx.SqlConn
	now  func() time.Time
}

type postgresMessageRow struct {
	ServerMsgID    string    `db:"server_msg_id"`
	ClientMsgID    string    `db:"client_msg_id"`
	SenderID       string    `db:"sender_id"`
	ConversationID string    `db:"conversation_id"`
	Seq            int64     `db:"seq"`
	ChatType       string    `db:"chat_type"`
	ReceiverID     string    `db:"receiver_id"`
	GroupID        string    `db:"group_id"`
	ContentType    string    `db:"content_type"`
	Content        []byte    `db:"content"`
	PayloadHash    string    `db:"payload_hash"`
	SendTime       time.Time `db:"send_time"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

type postgresMessageIdempotencyRow struct {
	PayloadHash    string `db:"payload_hash"`
	ServerMsgID    string `db:"server_msg_id"`
	ConversationID string `db:"conversation_id"`
	Seq            int64  `db:"seq"`
}

type postgresConversationLockRow struct {
	MaxSeq int64 `db:"max_seq"`
}

type postgresConversationStateRow struct {
	ConversationID string       `db:"conversation_id"`
	MaxSeq         int64        `db:"max_seq"`
	HasReadSeq     int64        `db:"has_read_seq"`
	MaxSeqTime     sql.NullTime `db:"max_seq_time"`
	LastMessageID  string       `db:"last_message_id"`
	UpdatedAt      time.Time    `db:"updated_at"`
}

func NewPostgresMessageRepository(dataSource string) (*PostgresMessageRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresMessageRepositoryFromConn(postgres.New(dataSource)), nil
}

func NewPostgresMessageRepositoryFromConn(conn sqlx.SqlConn) *PostgresMessageRepository {
	return &PostgresMessageRepository{conn: conn, now: time.Now}
}

func (r *PostgresMessageRepository) CreateMessageIdempotent(ctx context.Context, input CreateMessageInput) (Message, bool, error) {
	conversationID, err := inputConversationID(input)
	if err != nil {
		return Message{}, false, err
	}
	payloadHash := messagePayloadHash(input, conversationID)

	var stored Message
	deduplicated := false
	err = r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		existing, err := queryMessageIdempotency(ctx, session, input.SenderID, input.ClientMsgID)
		if err == nil {
			if existing.PayloadHash != payloadHash {
				return apperror.AlreadyExists("idempotency conflict")
			}
			message, err := queryMessageByServerID(ctx, session, existing.ServerMsgID)
			if err != nil {
				return err
			}
			stored = message.message()
			deduplicated = true
			return nil
		}
		if err != nil && !isNotFound(err) {
			return err
		}

		thread, err := upsertAndLockConversation(ctx, session, conversationID, input)
		if err != nil {
			return err
		}
		nextSeq := thread.MaxSeq + 1
		sendTime := r.now().UTC()

		messageRow, err := insertMessage(ctx, session, input, conversationID, nextSeq, payloadHash, sendTime)
		if err != nil {
			return err
		}
		if err := insertMessageIdempotency(ctx, session, input, messageRow); err != nil {
			return err
		}
		if err := upsertVisibleConversationStates(ctx, session, input, conversationID, nextSeq); err != nil {
			return err
		}
		if err := upsertSenderReadState(ctx, session, input.SenderID, conversationID, nextSeq); err != nil {
			return err
		}
		if err := updateConversationThreadAfterMessage(ctx, session, conversationID, messageRow.ServerMsgID, nextSeq, sendTime); err != nil {
			return err
		}
		if err := insertDeliveryAttemptsForMessage(ctx, session, messageRow, input); err != nil {
			return err
		}
		if err := insertMessageOutboxEvent(ctx, session, messageRow, input); err != nil {
			return err
		}

		stored = messageRow.message()
		return nil
	})
	if err != nil {
		if isPostgresUniqueViolation(err) {
			message, deduplicated, dedupeErr := r.existingMessageForIdempotency(ctx, input, payloadHash)
			if dedupeErr != nil {
				return Message{}, false, dedupeErr
			}
			return message, deduplicated, nil
		}
		if isPostgresCheckViolation(err) {
			return Message{}, false, apperror.InvalidArgument("invalid message")
		}
		return Message{}, false, err
	}

	if stored.ServerMsgID == "" {
		message, deduplicated, err := r.existingMessageForIdempotency(ctx, input, payloadHash)
		return message, deduplicated, err
	}
	return stored.Clone(), deduplicated, nil
}

func (r *PostgresMessageRepository) GetMessages(ctx context.Context, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error) {
	var maxSeq int64
	if err := r.conn.QueryRowCtx(ctx, &maxSeq, `
select max_seq from conversation_threads where conversation_id = $1
`, conversationID); err != nil {
		if isNotFound(err) {
			return nil, false, 0, apperror.NotFound("conversation not found")
		}
		return nil, false, 0, err
	}

	if fromSeq <= 0 {
		fromSeq = 1
	}
	if toSeq <= 0 || toSeq > maxSeq {
		toSeq = maxSeq
	}
	if limit <= 0 {
		limit = 50
	}
	if fromSeq > toSeq || maxSeq == 0 {
		return []Message{}, true, fromSeq, nil
	}

	order = strings.ToLower(strings.TrimSpace(order))
	if order == "" {
		order = MessageStorageOrderAsc
	}
	if order != MessageStorageOrderAsc && order != MessageStorageOrderDesc {
		return nil, false, 0, apperror.InvalidArgument("order must be asc or desc")
	}

	query := `
select server_msg_id, client_msg_id, sender_id, conversation_id, seq, chat_type,
       receiver_id, group_id, content_type, content, payload_hash, send_time, created_at, updated_at
from messages
where conversation_id = $1 and seq >= $2 and seq <= $3
order by seq ` + order + `
limit $4
`
	var rows []postgresMessageRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, query, conversationID, fromSeq, toSeq, limit+1); err != nil {
		return nil, false, 0, err
	}

	isEnd := true
	if len(rows) > limit {
		isEnd = false
		rows = rows[:limit]
	}
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, row.message())
	}

	nextSeq := fromSeq
	if len(messages) > 0 {
		if order == MessageStorageOrderDesc {
			nextSeq = messages[len(messages)-1].Seq - 1
		} else {
			nextSeq = messages[len(messages)-1].Seq + 1
		}
	}
	return messages, isEnd, nextSeq, nil
}

func (r *PostgresMessageRepository) GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error) {
	ids := conversationIDs
	if len(ids) == 0 {
		if err := r.conn.QueryRowsCtx(ctx, &ids, `
select conversation_id
from user_conversation_states
where user_id = $1
order by updated_at desc, conversation_id asc
`, userID); err != nil {
			return nil, err
		}
	}

	states := make([]ConversationSeqState, 0, len(ids))
	for _, conversationID := range ids {
		state, err := queryConversationSeqState(ctx, r.conn, userID, conversationID)
		if err != nil {
			if isNotFound(err) {
				return nil, apperror.NotFound("conversation not found")
			}
			return nil, err
		}
		states = append(states, state.Clone())
	}
	return states, nil
}

func (r *PostgresMessageRepository) SetUserHasReadSeqMax(ctx context.Context, userID, conversationID string, seq int64) (ConversationSeqState, bool, error) {
	if seq < 0 {
		return ConversationSeqState{}, false, apperror.InvalidArgument("has_read_seq must be greater than or equal to 0")
	}

	var state ConversationSeqState
	updated := false
	err := r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var stateRow struct {
			HasReadSeq     int64 `db:"has_read_seq"`
			LastVisibleSeq int64 `db:"last_visible_seq"`
		}
		if err := session.QueryRowCtx(ctx, &stateRow, `
select has_read_seq, last_visible_seq
from user_conversation_states
where user_id = $1 and conversation_id = $2
for update
`, userID, conversationID); err != nil {
			return err
		}
		if seq > stateRow.LastVisibleSeq {
			return apperror.InvalidArgument("has_read_seq cannot exceed max_seq")
		}

		updated = seq > stateRow.HasReadSeq
		if _, err := session.ExecCtx(ctx, `
update user_conversation_states
set has_read_seq = greatest(has_read_seq, $3),
    last_visible_seq = greatest(last_visible_seq, $3),
    updated_at = now()
where user_id = $1 and conversation_id = $2
`, userID, conversationID, seq); err != nil {
			return err
		}

		nextState, err := queryConversationSeqState(ctx, session, userID, conversationID)
		if err != nil {
			return err
		}
		state = nextState
		return nil
	})
	if err != nil {
		if isNotFound(err) {
			return ConversationSeqState{}, false, apperror.NotFound("conversation not found")
		}
		return ConversationSeqState{}, false, err
	}
	return state.Clone(), updated, nil
}

func queryMessageIdempotency(ctx context.Context, session sqlx.Session, senderID string, clientMsgID string) (postgresMessageIdempotencyRow, error) {
	var row postgresMessageIdempotencyRow
	err := session.QueryRowCtx(ctx, &row, `
select payload_hash, server_msg_id, conversation_id, seq
from message_idempotency_keys
where sender_id = $1 and client_msg_id = $2
`, senderID, clientMsgID)
	return row, err
}

func upsertAndLockConversation(ctx context.Context, session sqlx.Session, conversationID string, input CreateMessageInput) (postgresConversationLockRow, error) {
	switch input.ChatType {
	case ChatTypeSingle:
		userA, userB := MessageStorageOrderedSingleUsers(input.SenderID, input.ReceiverID)
		if _, err := session.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, chat_type, single_user_a, single_user_b)
values ($1, $2, $3, $4)
on conflict (conversation_id) do nothing
`, conversationID, input.ChatType, userA, userB); err != nil {
			return postgresConversationLockRow{}, err
		}
	case ChatTypeGroup:
		if _, err := session.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, chat_type, group_id)
values ($1, $2, $3)
on conflict (conversation_id) do nothing
`, conversationID, input.ChatType, input.GroupID); err != nil {
			return postgresConversationLockRow{}, err
		}
	default:
		return postgresConversationLockRow{}, apperror.InvalidArgument("chat_type must be single or group")
	}

	var row postgresConversationLockRow
	err := session.QueryRowCtx(ctx, &row, `
select max_seq
from conversation_threads
where conversation_id = $1
for update
`, conversationID)
	return row, err
}

func insertMessage(ctx context.Context, session sqlx.Session, input CreateMessageInput, conversationID string, seq int64, payloadHash string, sendTime time.Time) (postgresMessageRow, error) {
	contentJSON, err := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: input.Content})
	if err != nil {
		return postgresMessageRow{}, err
	}

	var row postgresMessageRow
	err = session.QueryRowCtx(ctx, &row, `
insert into messages (
  client_msg_id, sender_id, conversation_id, seq, chat_type,
  receiver_id, group_id, content_type, content, payload_hash, send_time
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11)
returning server_msg_id, client_msg_id, sender_id, conversation_id, seq, chat_type,
          receiver_id, group_id, content_type, content, payload_hash, send_time, created_at, updated_at
`, input.ClientMsgID, input.SenderID, conversationID, seq, input.ChatType, input.ReceiverID, input.GroupID,
		input.ContentType, string(contentJSON), payloadHash, sendTime)
	return row, err
}

func insertMessageIdempotency(ctx context.Context, session sqlx.Session, input CreateMessageInput, message postgresMessageRow) error {
	_, err := session.ExecCtx(ctx, `
insert into message_idempotency_keys (
  sender_id, client_msg_id, payload_hash, server_msg_id, conversation_id, seq, status
)
values ($1, $2, $3, $4, $5, $6, $7)
`, input.SenderID, input.ClientMsgID, message.PayloadHash, message.ServerMsgID, message.ConversationID, message.Seq, "accepted")
	return err
}

func upsertVisibleConversationStates(ctx context.Context, session sqlx.Session, input CreateMessageInput, conversationID string, seq int64) error {
	for _, userID := range visibleUserIDs(input) {
		if _, err := session.ExecCtx(ctx, `
insert into user_conversation_states (user_id, conversation_id, last_visible_seq)
values ($1, $2, $3)
on conflict (user_id, conversation_id) do update
set last_visible_seq = greatest(user_conversation_states.last_visible_seq, excluded.last_visible_seq),
    updated_at = now()
`, userID, conversationID, seq); err != nil {
			return err
		}
	}
	return nil
}

func upsertSenderReadState(ctx context.Context, session sqlx.Session, senderID string, conversationID string, seq int64) error {
	_, err := session.ExecCtx(ctx, `
insert into user_conversation_states (user_id, conversation_id, has_read_seq, last_visible_seq)
values ($1, $2, $3, $3)
on conflict (user_id, conversation_id) do update
set has_read_seq = greatest(user_conversation_states.has_read_seq, excluded.has_read_seq),
    last_visible_seq = greatest(user_conversation_states.last_visible_seq, excluded.last_visible_seq),
    updated_at = now()
`, senderID, conversationID, seq)
	return err
}

func updateConversationThreadAfterMessage(ctx context.Context, session sqlx.Session, conversationID string, serverMsgID string, seq int64, sendTime time.Time) error {
	_, err := session.ExecCtx(ctx, `
update conversation_threads
set max_seq = $2,
    last_message_id = $3,
    last_message_at = $4,
    updated_at = now()
where conversation_id = $1
`, conversationID, seq, serverMsgID, sendTime)
	return err
}

func queryConversationSeqState(ctx context.Context, session sqlx.Session, userID string, conversationID string) (ConversationSeqState, error) {
	var row postgresConversationStateRow
	if err := session.QueryRowCtx(ctx, &row, `
select t.conversation_id,
       s.last_visible_seq as max_seq,
       s.has_read_seq,
       m.send_time as max_seq_time,
       coalesce(m.server_msg_id, '') as last_message_id,
       s.updated_at
from conversation_threads t
join user_conversation_states s on s.conversation_id = t.conversation_id
left join messages m on m.conversation_id = t.conversation_id and m.seq = s.last_visible_seq
where s.user_id = $1 and t.conversation_id = $2
`, userID, conversationID); err != nil {
		return ConversationSeqState{}, err
	}

	state := ConversationSeqState{
		ConversationID: row.ConversationID,
		MaxSeq:         row.MaxSeq,
		HasReadSeq:     row.HasReadSeq,
		UnreadCount:    MessageStorageUnreadCount(row.MaxSeq, row.HasReadSeq),
	}
	if row.MaxSeqTime.Valid {
		state.MaxSeqTime = row.MaxSeqTime.Time.UTC().UnixMilli()
	}
	if row.LastMessageID != "" {
		lastMessage, err := queryMessageByServerID(ctx, session, row.LastMessageID)
		if err != nil {
			return ConversationSeqState{}, err
		}
		message := lastMessage.message()
		state.LastMessage = &message
	}
	return state, nil
}

func queryMessageByServerID(ctx context.Context, session sqlx.Session, serverMsgID string) (postgresMessageRow, error) {
	var row postgresMessageRow
	err := session.QueryRowCtx(ctx, &row, `
select server_msg_id, client_msg_id, sender_id, conversation_id, seq, chat_type,
       receiver_id, group_id, content_type, content, payload_hash, send_time, created_at, updated_at
from messages
where server_msg_id = $1
`, serverMsgID)
	return row, err
}

func queryMessageBySenderClient(ctx context.Context, session sqlx.Session, senderID string, clientMsgID string) (postgresMessageRow, error) {
	var row postgresMessageRow
	err := session.QueryRowCtx(ctx, &row, `
select server_msg_id, client_msg_id, sender_id, conversation_id, seq, chat_type,
       receiver_id, group_id, content_type, content, payload_hash, send_time, created_at, updated_at
from messages
where sender_id = $1 and client_msg_id = $2
`, senderID, clientMsgID)
	return row, err
}

func (r *PostgresMessageRepository) existingMessageForIdempotency(ctx context.Context, input CreateMessageInput, payloadHash string) (Message, bool, error) {
	existing, err := queryMessageIdempotency(ctx, r.conn, input.SenderID, input.ClientMsgID)
	if err == nil {
		if existing.PayloadHash != payloadHash {
			return Message{}, false, apperror.AlreadyExists("idempotency conflict")
		}
		message, err := queryMessageByServerID(ctx, r.conn, existing.ServerMsgID)
		if err != nil {
			return Message{}, false, err
		}
		return message.message(), true, nil
	}
	if err != nil && !isNotFound(err) {
		return Message{}, false, err
	}

	message, err := queryMessageBySenderClient(ctx, r.conn, input.SenderID, input.ClientMsgID)
	if err != nil {
		if isNotFound(err) {
			return Message{}, false, apperror.AlreadyExists("idempotency conflict")
		}
		return Message{}, false, err
	}
	if message.PayloadHash != payloadHash {
		return Message{}, false, apperror.AlreadyExists("idempotency conflict")
	}
	return message.message(), true, nil
}

func messagePayloadHash(input CreateMessageInput, conversationID string) string {
	payload := struct {
		SenderID       string `json:"sender_id"`
		ClientMsgID    string `json:"client_msg_id"`
		ConversationID string `json:"conversation_id"`
		ChatType       string `json:"chat_type"`
		ReceiverID     string `json:"receiver_id"`
		GroupID        string `json:"group_id"`
		ContentType    string `json:"content_type"`
		Content        string `json:"content"`
	}{
		SenderID:       input.SenderID,
		ClientMsgID:    input.ClientMsgID,
		ConversationID: conversationID,
		ChatType:       input.ChatType,
		ReceiverID:     input.ReceiverID,
		GroupID:        input.GroupID,
		ContentType:    input.ContentType,
		Content:        input.Content,
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func visibleUserIDs(input CreateMessageInput) []string {
	seen := make(map[string]struct{})
	add := func(userID string) {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return
		}
		seen[userID] = struct{}{}
	}
	for _, userID := range input.ParticipantUserIDs {
		add(userID)
	}
	add(input.SenderID)
	if input.ChatType == ChatTypeSingle {
		add(input.ReceiverID)
	}

	users := make([]string, 0, len(seen))
	for userID := range seen {
		users = append(users, userID)
	}
	return users
}

func (r postgresMessageRow) message() Message {
	return Message{
		ServerMsgID:    r.ServerMsgID,
		ClientMsgID:    r.ClientMsgID,
		ConversationID: r.ConversationID,
		Seq:            r.Seq,
		SenderID:       r.SenderID,
		ReceiverID:     r.ReceiverID,
		GroupID:        r.GroupID,
		ChatType:       r.ChatType,
		ContentType:    r.ContentType,
		Content:        decodeMessageContent(r.Content),
		SendTime:       r.SendTime.UTC().UnixMilli(),
		CreatedAt:      r.CreatedAt.UTC().UnixMilli(),
	}
}

func decodeMessageContent(content []byte) string {
	var textBody struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &textBody); err == nil && textBody.Text != "" {
		return textBody.Text
	}
	return string(content)
}
