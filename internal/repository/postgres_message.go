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
	"github.com/wujunhui99/agents_im/internal/idgen"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PostgresMessageRepository struct {
	conn sqlx.SqlConn
	now  func() time.Time
}

type postgresMessageRow struct {
	ServerMsgID           string    `db:"message_id"`
	ClientMsgID           string    `db:"client_msg_id"`
	SenderID              string    `db:"sender_account_id"`
	ConversationID        string    `db:"conversation_id"`
	Seq                   int64     `db:"seq"`
	ConversationType      int16     `db:"conversation_type"`
	ReceiverID            string    `db:"receiver_account_id"`
	GroupID               string    `db:"group_id"`
	ContentTypeValue      int16     `db:"content_type"`
	Content               []byte    `db:"content"`
	MessageOriginValue    int16     `db:"message_origin"`
	AgentAccountID        string    `db:"agent_account_id"`
	TriggerServerMsgID    string    `db:"trigger_message_id"`
	AgentRunID            string    `db:"agent_run_id"`
	AllowRecursiveTrigger bool      `db:"allow_recursive_trigger"`
	PayloadHash           string    `db:"payload_hash"`
	SendTime              time.Time `db:"client_send_time"`
	CreatedAt             time.Time `db:"server_received_at"`
	UpdatedAt             time.Time `db:"updated_at"`
}

type postgresConversationLockRow struct {
	MaxSeq int64 `db:"max_seq"`
}

type postgresConversationStateRow struct {
	ConversationID  string       `db:"conversation_id"`
	MaxSeq          int64        `db:"max_seq"`
	HasReadSeq      int64        `db:"has_read_seq"`
	VisibleStartSeq int64        `db:"visible_start_seq"`
	MaxSeqTime      sql.NullTime `db:"max_seq_time"`
	LastMessageID   string       `db:"last_message_id"`
	UpdatedAt       time.Time    `db:"updated_at"`
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
	if _, err := normalizeMessageOriginInput(&input); err != nil {
		return Message{}, false, err
	}
	conversationID, err := validateCreateMessageInput(input)
	if err != nil {
		return Message{}, false, err
	}
	payloadHash := messagePayloadHash(input, conversationID)

	var stored Message
	deduplicated := false
	err = r.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		existing, err := queryMessageBySenderClient(ctx, session, input.SenderID, input.ClientMsgID)
		if err == nil {
			if existing.PayloadHash != payloadHash {
				return apperror.AlreadyExists("idempotency conflict")
			}
			stored = existing.message()
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
		if err := upsertVisibleConversationStates(ctx, session, input, conversationID, thread.MaxSeq); err != nil {
			return err
		}
		if err := upsertSenderReadState(ctx, session, input.SenderID, conversationID, nextSeq); err != nil {
			return err
		}
		if err := updateConversationThreadAfterMessage(ctx, session, conversationID, messageRow.ServerMsgID, nextSeq, sendTime); err != nil {
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
	var err error
	fromSeq, toSeq, limit, order, err = normalizeMessagePullRange(fromSeq, toSeq, limit, order)
	if err != nil {
		return nil, false, 0, err
	}

	var maxSeq int64
	if err := r.conn.QueryRowCtx(ctx, &maxSeq, `
select max_seq from conversation_threads where conversation_id = $1
`, conversationID); err != nil {
		if isNotFound(err) {
			return nil, false, 0, apperror.NotFound("conversation not found")
		}
		return nil, false, 0, err
	}

	return r.getMessagesInRange(ctx, conversationID, fromSeq, toSeq, maxSeq, limit, order)
}

func (r *PostgresMessageRepository) getMessagesInRange(ctx context.Context, conversationID string, fromSeq, toSeq int64, maxSeq int64, limit int, order string) ([]Message, bool, int64, error) {
	if toSeq <= 0 || toSeq > maxSeq {
		toSeq = maxSeq
	}
	if fromSeq > toSeq || maxSeq == 0 {
		return []Message{}, true, fromSeq, nil
	}

	query := `
select message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
       receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
       trigger_message_id, agent_run_id, allow_recursive_trigger,
       payload_hash, coalesce(client_send_time, server_received_at) as client_send_time,
       server_received_at, updated_at
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

func (r *PostgresMessageRepository) GetMessagesForUser(ctx context.Context, userID string, conversationID string, fromSeq, toSeq int64, limit int, order string) ([]Message, bool, int64, error) {
	var err error
	fromSeq, toSeq, limit, order, err = normalizeMessagePullRange(fromSeq, toSeq, limit, order)
	if err != nil {
		return nil, false, 0, err
	}

	var state struct {
		MaxSeq          int64 `db:"max_seq"`
		VisibleStartSeq int64 `db:"visible_start_seq"`
	}
	if err := r.conn.QueryRowCtx(ctx, &state, `
select t.max_seq, s.visible_start_seq
from conversation_threads t
join user_conversation_states s on s.conversation_id = t.conversation_id
where s.account_id = $1 and t.conversation_id = $2
`, userID, conversationID); err != nil {
		if isNotFound(err) {
			return nil, false, 0, apperror.NotFound("conversation not found")
		}
		return nil, false, 0, err
	}

	if fromSeq <= state.VisibleStartSeq {
		fromSeq = state.VisibleStartSeq + 1
	}
	if toSeq <= 0 || toSeq > state.MaxSeq {
		toSeq = state.MaxSeq
	}
	return r.getMessagesInRange(ctx, conversationID, fromSeq, toSeq, state.MaxSeq, limit, order)
}

func (r *PostgresMessageRepository) GetConversationSeqStates(ctx context.Context, userID string, conversationIDs []string) ([]ConversationSeqState, error) {
	ids := conversationIDs
	if len(ids) == 0 {
		if err := r.conn.QueryRowsCtx(ctx, &ids, `
select conversation_id
from user_conversation_states
where account_id = $1
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
			HasReadSeq      int64 `db:"last_read_seq"`
			MaxSeq          int64 `db:"max_seq"`
			VisibleStartSeq int64 `db:"visible_start_seq"`
		}
		if err := session.QueryRowCtx(ctx, &stateRow, `
select s.last_read_seq, s.visible_start_seq, t.max_seq
from user_conversation_states s
join conversation_threads t on t.conversation_id = s.conversation_id
where s.account_id = $1 and s.conversation_id = $2
for update
`, userID, conversationID); err != nil {
			return err
		}
		if seq > stateRow.MaxSeq {
			return apperror.InvalidArgument("has_read_seq cannot exceed max_seq")
		}
		if seq < stateRow.VisibleStartSeq {
			seq = stateRow.VisibleStartSeq
		}

		updated = seq > stateRow.HasReadSeq
		if _, err := session.ExecCtx(ctx, `
update user_conversation_states
set last_read_seq = greatest(last_read_seq, $3, visible_start_seq),
    updated_at = now()
where account_id = $1 and conversation_id = $2
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

func upsertAndLockConversation(ctx context.Context, session sqlx.Session, conversationID string, input CreateMessageInput) (postgresConversationLockRow, error) {
	conversationType, err := conversationTypeValue(input.ChatType)
	if err != nil {
		return postgresConversationLockRow{}, err
	}
	switch input.ChatType {
	case ChatTypeSingle:
		userA, userB := MessageStorageOrderedSingleUsers(input.SenderID, input.ReceiverID)
		if _, err := session.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, conversation_type, single_account_a, single_account_b)
values ($1, $2, $3, $4)
on conflict (conversation_id) do nothing
`, conversationID, conversationType, userA, userB); err != nil {
			return postgresConversationLockRow{}, err
		}
	case ChatTypeGroup:
		if _, err := session.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, conversation_type, group_id)
values ($1, $2, $3)
on conflict (conversation_id) do nothing
`, conversationID, conversationType, input.GroupID); err != nil {
			return postgresConversationLockRow{}, err
		}
	default:
		return postgresConversationLockRow{}, apperror.InvalidArgument("chat_type must be single or group")
	}

	var row postgresConversationLockRow
	err = session.QueryRowCtx(ctx, &row, `
select max_seq
from conversation_threads
where conversation_id = $1
for update
`, conversationID)
	return row, err
}

func insertMessage(ctx context.Context, session sqlx.Session, input CreateMessageInput, conversationID string, seq int64, payloadHash string, sendTime time.Time) (postgresMessageRow, error) {
	contentJSON, err := messageContentJSON(input)
	if err != nil {
		return postgresMessageRow{}, err
	}
	conversationType, err := conversationTypeValue(input.ChatType)
	if err != nil {
		return postgresMessageRow{}, err
	}
	contentType, err := contentTypeValue(input.ContentType)
	if err != nil {
		return postgresMessageRow{}, err
	}
	messageID, err := idgen.NewString()
	if err != nil {
		return postgresMessageRow{}, err
	}
	messageOrigin, err := messageOriginValue(input.MessageOrigin)
	if err != nil {
		return postgresMessageRow{}, err
	}

	var row postgresMessageRow
	err = session.QueryRowCtx(ctx, &row, `
insert into messages (
	  message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
	  receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
	  trigger_message_id, agent_run_id, allow_recursive_trigger, payload_hash, client_send_time
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
returning message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
	      receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
	      trigger_message_id, agent_run_id, allow_recursive_trigger,
	      payload_hash, coalesce(client_send_time, server_received_at) as client_send_time,
	      server_received_at, updated_at
`, messageID, input.ClientMsgID, input.SenderID, conversationID, seq, conversationType, input.ReceiverID, input.GroupID,
		contentType, string(contentJSON), messageOrigin, input.AgentAccountID, input.TriggerServerMsgID,
		input.AgentRunID, input.AllowRecursiveTrigger, payloadHash, sendTime)
	return row, err
}

func messageContentJSON(input CreateMessageInput) ([]byte, error) {
	if input.ContentType == ContentTypeText {
		return json.Marshal(struct {
			Text string `json:"text"`
		}{Text: input.Content})
	}
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(input.Content), &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func upsertVisibleConversationStates(ctx context.Context, session sqlx.Session, input CreateMessageInput, conversationID string, previousMaxSeq int64) error {
	visibleStartSeq := int64(0)
	if input.ChatType == ChatTypeGroup {
		visibleStartSeq = previousMaxSeq
	}
	for _, userID := range visibleUserIDs(input) {
		if _, err := session.ExecCtx(ctx, `
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

func upsertSenderReadState(ctx context.Context, session sqlx.Session, senderID string, conversationID string, seq int64) error {
	_, err := session.ExecCtx(ctx, `
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
values ($1, $2, $3, 0)
on conflict (account_id, conversation_id) do update
set last_read_seq = greatest(user_conversation_states.last_read_seq, excluded.last_read_seq),
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
	       t.max_seq,
	       s.last_read_seq as has_read_seq,
	       s.visible_start_seq,
	       lm.client_send_time as max_seq_time,
	       case when t.max_seq > s.visible_start_seq then coalesce(t.last_message_id, '') else '' end as last_message_id,
	       s.updated_at
from conversation_threads t
join user_conversation_states s on s.conversation_id = t.conversation_id
left join messages lm on lm.message_id = t.last_message_id
where s.account_id = $1 and t.conversation_id = $2
`, userID, conversationID); err != nil {
		return ConversationSeqState{}, err
	}

	state := ConversationSeqState{
		ConversationID: row.ConversationID,
		MaxSeq:         row.MaxSeq,
		HasReadSeq:     row.HasReadSeq,
		UnreadCount:    MessageStorageUnreadCountFromVisibleStart(row.MaxSeq, row.HasReadSeq, row.VisibleStartSeq),
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
select message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
	       receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
	       trigger_message_id, agent_run_id, allow_recursive_trigger,
	       payload_hash, coalesce(client_send_time, server_received_at) as client_send_time,
	       server_received_at, updated_at
from messages
where message_id = $1
`, serverMsgID)
	return row, err
}

func queryMessageBySenderClient(ctx context.Context, session sqlx.Session, senderID string, clientMsgID string) (postgresMessageRow, error) {
	var row postgresMessageRow
	err := session.QueryRowCtx(ctx, &row, `
select message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
	       receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
	       trigger_message_id, agent_run_id, allow_recursive_trigger,
	       payload_hash, coalesce(client_send_time, server_received_at) as client_send_time,
	       server_received_at, updated_at
from messages
where sender_account_id = $1 and client_msg_id = $2
`, senderID, clientMsgID)
	return row, err
}

func (r *PostgresMessageRepository) existingMessageForIdempotency(ctx context.Context, input CreateMessageInput, payloadHash string) (Message, bool, error) {
	existing, err := queryMessageBySenderClient(ctx, r.conn, input.SenderID, input.ClientMsgID)
	if err == nil {
		if existing.PayloadHash != payloadHash {
			return Message{}, false, apperror.AlreadyExists("idempotency conflict")
		}
		return existing.message(), true, nil
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
		SenderID              string `json:"sender_id"`
		ClientMsgID           string `json:"client_msg_id"`
		ConversationID        string `json:"conversation_id"`
		ChatType              string `json:"chat_type"`
		ReceiverID            string `json:"receiver_id"`
		GroupID               string `json:"group_id"`
		ContentType           string `json:"content_type"`
		Content               string `json:"content"`
		MessageOrigin         string `json:"message_origin"`
		AgentAccountID        string `json:"agent_account_id,omitempty"`
		TriggerServerMsgID    string `json:"trigger_server_msg_id,omitempty"`
		AgentRunID            string `json:"agent_run_id,omitempty"`
		AllowRecursiveTrigger bool   `json:"allow_recursive_trigger,omitempty"`
	}{
		SenderID:              input.SenderID,
		ClientMsgID:           input.ClientMsgID,
		ConversationID:        conversationID,
		ChatType:              input.ChatType,
		ReceiverID:            input.ReceiverID,
		GroupID:               input.GroupID,
		ContentType:           input.ContentType,
		Content:               input.Content,
		MessageOrigin:         input.MessageOrigin,
		AgentAccountID:        input.AgentAccountID,
		TriggerServerMsgID:    input.TriggerServerMsgID,
		AgentRunID:            input.AgentRunID,
		AllowRecursiveTrigger: input.AllowRecursiveTrigger,
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
		ServerMsgID:           r.ServerMsgID,
		ClientMsgID:           r.ClientMsgID,
		ConversationID:        r.ConversationID,
		Seq:                   r.Seq,
		SenderID:              r.SenderID,
		ReceiverID:            r.ReceiverID,
		GroupID:               r.GroupID,
		ChatType:              conversationTypeString(r.ConversationType),
		ContentType:           contentTypeString(r.ContentTypeValue),
		Content:               decodeMessageContent(r.Content),
		MessageOrigin:         messageOriginString(r.MessageOriginValue),
		AgentAccountID:        r.AgentAccountID,
		TriggerServerMsgID:    r.TriggerServerMsgID,
		AgentRunID:            r.AgentRunID,
		AllowRecursiveTrigger: r.AllowRecursiveTrigger,
		SendTime:              r.SendTime.UTC().UnixMilli(),
		CreatedAt:             r.CreatedAt.UTC().UnixMilli(),
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
