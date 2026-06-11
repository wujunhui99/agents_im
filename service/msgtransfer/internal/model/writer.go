// Package model is msgtransfer's PostgreSQL data layer (03-message-pipeline §9 B1).
// It is handwritten rather than goctl-generated: this worker owns no CRUD surface,
// only the fixed write-set of the async persist consumer (thread upsert, message
// insert with pre-assigned seq, visibility/read-state upserts) plus the seq-seed
// query. SQL semantics mirror service/msg/rpc/internal/model — the same rows the
// synchronous path writes today.
package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ChatTypeSingle = "single"
	ChatTypeGroup  = "group"

	conversationTypeSingle int64 = 1
	conversationTypeGroup  int64 = 2

	contentTypeTextValue  int64 = 1
	contentTypeImageValue int64 = 2
	contentTypeFileValue  int64 = 3

	messageOriginHumanValue  int64 = 1
	messageOriginAIValue     int64 = 2
	messageOriginSystemValue int64 = 3
)

// PersistMessage is one seq-assigned message bound for the messages table.
type PersistMessage struct {
	ServerMsgID           string
	ClientMsgID           string
	SenderID              string
	ConversationID        string
	Seq                   int64
	ChatType              string
	ReceiverID            string
	GroupID               string
	ContentType           string
	Content               string // canonical JSON for the jsonb column
	MessageOrigin         string
	AgentAccountID        string
	TriggerServerMsgID    string
	AgentRunID            string
	AllowRecursiveTrigger bool
	PayloadHash           string
	SendTime              time.Time
	VisibleUserIDs        []string
}

type Writer struct {
	conn sqlx.SqlConn
}

func NewWriter(dataSource string) (*Writer, error) {
	if strings.TrimSpace(dataSource) == "" {
		return nil, errors.New("msgtransfer model requires a postgres data source")
	}
	return &Writer{conn: postgres.New(dataSource)}, nil
}

// MaxSeq seeds the Redis seq counter: durable max(seq) for the conversation.
func (w *Writer) MaxSeq(ctx context.Context, conversationID string) (int64, error) {
	var maxSeq int64
	err := w.conn.QueryRowCtx(ctx, &maxSeq,
		`select coalesce(max(seq), 0) from messages where conversation_id = $1`, conversationID)
	if err != nil {
		return 0, fmt.Errorf("query max seq: %w", err)
	}
	return maxSeq, nil
}

// PersistBatch writes one conversation's ordered batch in a single transaction.
// Replays converge via ON CONFLICT (sender_account_id, client_msg_id) DO NOTHING.
// A (conversation_id, seq) unique violation is NOT swallowed: it means the Redis
// seq counter regressed (e.g. AOF loss) — the consumer must fail loudly and keep
// retrying until the operator advances the counter (03 §9 B0 deviation note).
func (w *Writer) PersistBatch(ctx context.Context, msgs []PersistMessage) error {
	if len(msgs) == 0 {
		return nil
	}
	return w.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, msg := range msgs {
			if err := upsertThread(ctx, session, msg); err != nil {
				return fmt.Errorf("upsert thread %s: %w", msg.ConversationID, err)
			}
			// Replays no-op here; the state upserts below are themselves
			// idempotent and re-run to converge a partially-applied earlier tx.
			if err := insertMessage(ctx, session, msg); err != nil {
				return fmt.Errorf("insert message %s seq %d: %w", msg.ServerMsgID, msg.Seq, err)
			}
		}
		for _, msg := range msgs {
			visibleStartSeq := int64(0)
			if msg.ChatType == ChatTypeGroup {
				visibleStartSeq = msg.Seq - 1
			}
			if err := upsertVisible(ctx, session, msg.VisibleUserIDs, msg.ConversationID, visibleStartSeq); err != nil {
				return fmt.Errorf("upsert visible %s: %w", msg.ConversationID, err)
			}
			if err := upsertSenderRead(ctx, session, msg.SenderID, msg.ConversationID, msg.Seq); err != nil {
				return fmt.Errorf("upsert sender read %s: %w", msg.ConversationID, err)
			}
		}
		last := msgs[len(msgs)-1]
		if err := updateThreadAfterMessage(ctx, session, last); err != nil {
			return fmt.Errorf("update thread after message %s: %w", last.ConversationID, err)
		}
		return nil
	})
}

func upsertThread(ctx context.Context, session sqlx.Session, msg PersistMessage) error {
	switch msg.ChatType {
	case ChatTypeSingle:
		userA, userB := msg.SenderID, msg.ReceiverID
		if userA > userB {
			userA, userB = userB, userA
		}
		_, err := session.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, conversation_type, single_account_a, single_account_b)
values ($1, $2, $3, $4)
on conflict (conversation_id) do nothing
`, msg.ConversationID, conversationTypeSingle, userA, userB)
		return err
	case ChatTypeGroup:
		_, err := session.ExecCtx(ctx, `
insert into conversation_threads (conversation_id, conversation_type, group_id)
values ($1, $2, $3)
on conflict (conversation_id) do nothing
`, msg.ConversationID, conversationTypeGroup, msg.GroupID)
		return err
	default:
		return fmt.Errorf("unsupported chat_type %q", msg.ChatType)
	}
}

func insertMessage(ctx context.Context, session sqlx.Session, msg PersistMessage) error {
	conversationType, err := conversationTypeValue(msg.ChatType)
	if err != nil {
		return err
	}
	contentType, err := contentTypeValue(msg.ContentType)
	if err != nil {
		return err
	}
	messageOrigin, err := messageOriginValue(msg.MessageOrigin)
	if err != nil {
		return err
	}
	_, err = session.ExecCtx(ctx, `
insert into messages (
  message_id, client_msg_id, sender_account_id, conversation_id, seq, conversation_type,
  receiver_account_id, group_id, content_type, content, message_origin, agent_account_id,
  trigger_message_id, agent_run_id, allow_recursive_trigger, payload_hash, client_send_time
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
on conflict (sender_account_id, client_msg_id) do nothing
`, msg.ServerMsgID, msg.ClientMsgID, msg.SenderID, msg.ConversationID, msg.Seq, conversationType,
		msg.ReceiverID, msg.GroupID, contentType, msg.Content, messageOrigin, msg.AgentAccountID,
		msg.TriggerServerMsgID, msg.AgentRunID, msg.AllowRecursiveTrigger, msg.PayloadHash,
		sql.NullTime{Time: msg.SendTime, Valid: !msg.SendTime.IsZero()})
	return err
}

func upsertVisible(ctx context.Context, session sqlx.Session, userIDs []string, conversationID string, visibleStartSeq int64) error {
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
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

func upsertSenderRead(ctx context.Context, session sqlx.Session, senderID, conversationID string, seq int64) error {
	_, err := session.ExecCtx(ctx, `
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
values ($1, $2, $3, 0)
on conflict (account_id, conversation_id) do update
set last_read_seq = greatest(user_conversation_states.last_read_seq, excluded.last_read_seq),
    updated_at = now()
`, senderID, conversationID, seq)
	return err
}

// updateThreadAfterMessage advances max_seq/last_message monotonically so a
// replayed older batch can never roll the thread backwards.
func updateThreadAfterMessage(ctx context.Context, session sqlx.Session, msg PersistMessage) error {
	_, err := session.ExecCtx(ctx, `
update conversation_threads
set last_message_id = case when $2 >= max_seq then $3 else last_message_id end,
    last_message_at = case when $2 >= max_seq then $4 else last_message_at end,
    max_seq = greatest(max_seq, $2),
    updated_at = now()
where conversation_id = $1
`, msg.ConversationID, msg.Seq, msg.ServerMsgID, msg.SendTime.UTC())
	return err
}

func conversationTypeValue(chatType string) (int64, error) {
	switch strings.TrimSpace(strings.ToLower(chatType)) {
	case ChatTypeSingle:
		return conversationTypeSingle, nil
	case ChatTypeGroup:
		return conversationTypeGroup, nil
	default:
		return 0, fmt.Errorf("chat_type must be single or group, got %q", chatType)
	}
}

func contentTypeValue(contentType string) (int64, error) {
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case "", "text":
		return contentTypeTextValue, nil
	case "image":
		return contentTypeImageValue, nil
	case "file":
		return contentTypeFileValue, nil
	default:
		return 0, fmt.Errorf("content_type must be text, image, or file, got %q", contentType)
	}
}

func messageOriginValue(origin string) (int64, error) {
	switch strings.TrimSpace(strings.ToLower(origin)) {
	case "", "human":
		return messageOriginHumanValue, nil
	case "ai":
		return messageOriginAIValue, nil
	case "system":
		return messageOriginSystemValue, nil
	default:
		return 0, fmt.Errorf("message_origin must be human, ai, or system, got %q", origin)
	}
}

// IsUniqueViolation reports a Postgres 23505 — in the persist consumer this means
// the seq counter regressed (see PersistBatch doc).
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
