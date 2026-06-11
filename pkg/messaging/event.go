package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	DefaultMessageEventsTopic = "message.events.v1"
	DefaultConsumerGroup      = "msgtransfer-worker"
)

const (
	EventTypeMessageAccepted = "message.accepted"
	EventTypeMessageRead     = "message.read"
	// EventTypeMessageSubmitted is the msg.toTransfer.v1 payload: msg-rpc accepted
	// the send but seq is not yet allocated — msgtransfer assigns it (03 §4.2).
	EventTypeMessageSubmitted = "message.submitted"
)

const (
	ChatTypeSingle = "single"
	ChatTypeGroup  = "group"
)

type MessageEvent struct {
	EventID        string              `json:"event_id"`
	EventType      string              `json:"event_type"`
	ConversationID string              `json:"conversation_id"`
	ServerMsgID    string              `json:"server_msg_id"`
	Seq            int64               `json:"seq"`
	SenderID       string              `json:"sender_id"`
	ChatType       string              `json:"chat_type"`
	CreatedAt      int64               `json:"created_at"`
	Payload        MessageEventPayload `json:"payload"`
}

type MessageEventPayload struct {
	ClientMsgID           string          `json:"client_msg_id,omitempty"`
	ReceiverID            string          `json:"receiver_id,omitempty"`
	ReceiverIDs           []string        `json:"receiver_ids,omitempty"`
	GroupID               string          `json:"group_id,omitempty"`
	ContentType           string          `json:"content_type,omitempty"`
	Content               json.RawMessage `json:"content,omitempty"`
	MessageOrigin         string          `json:"message_origin,omitempty"`
	AgentAccountID        string          `json:"agent_account_id,omitempty"`
	TriggerServerMsgID    string          `json:"trigger_server_msg_id,omitempty"`
	AgentRunID            string          `json:"agent_run_id,omitempty"`
	AllowRecursiveTrigger bool            `json:"allow_recursive_trigger,omitempty"`
	UserID                string          `json:"user_id,omitempty"`
	HasReadSeq            int64           `json:"has_read_seq,omitempty"`
	ReadAt                int64           `json:"read_at,omitempty"`
	// VisibleUserIDs are the participants whose user_conversation_states rows must
	// exist (visibility), resolved by msg-rpc at send time. Required for
	// message.submitted; carried through to msg.toPostgres.v1.
	VisibleUserIDs []string `json:"visible_user_ids,omitempty"`
	// PayloadHash is msg-rpc's idempotency-conflict fingerprint, persisted to
	// messages.payload_hash so PG-path dedup semantics survive the Kafka path.
	PayloadHash string `json:"payload_hash,omitempty"`
	// SendTime is the authoritative message timestamp (unix ms) assigned by
	// msg-rpc; the persist consumer writes it to messages.client_send_time.
	SendTime    int64  `json:"send_time,omitempty"`
	TraceID     string `json:"trace_id,omitempty"`
	RequestID   string `json:"request_id,omitempty"`
	TraceParent string `json:"traceparent,omitempty"`
	TraceState  string `json:"tracestate,omitempty"`
}

func (e MessageEvent) Validate() error {
	if e.EventID == "" {
		return errors.New("event_id is required")
	}
	switch e.EventType {
	case EventTypeMessageAccepted, EventTypeMessageRead, EventTypeMessageSubmitted:
	default:
		return fmt.Errorf("unsupported event_type %q", e.EventType)
	}
	if e.ConversationID == "" {
		return errors.New("conversation_id is required")
	}
	if e.CreatedAt <= 0 {
		return errors.New("created_at must be a unix millisecond timestamp")
	}
	switch e.ChatType {
	case ChatTypeSingle, ChatTypeGroup:
	default:
		return fmt.Errorf("unsupported chat_type %q", e.ChatType)
	}
	if len(e.Payload.Content) > 0 && !json.Valid(e.Payload.Content) {
		return errors.New("payload.content must be valid JSON when set")
	}

	switch e.EventType {
	case EventTypeMessageAccepted:
		if e.ServerMsgID == "" {
			return errors.New("server_msg_id is required for message.accepted")
		}
		if e.Seq <= 0 {
			return errors.New("seq must be greater than 0 for message.accepted")
		}
		if e.SenderID == "" {
			return errors.New("sender_id is required for message.accepted")
		}
	case EventTypeMessageRead:
		if e.Seq < 0 {
			return errors.New("seq must be greater than or equal to 0 for message.read")
		}
		if e.Payload.UserID == "" {
			return errors.New("payload.user_id is required for message.read")
		}
	case EventTypeMessageSubmitted:
		if e.ServerMsgID == "" {
			return errors.New("server_msg_id is required for message.submitted")
		}
		if e.Seq != 0 {
			return errors.New("seq must be 0 for message.submitted (msgtransfer allocates it)")
		}
		if e.SenderID == "" {
			return errors.New("sender_id is required for message.submitted")
		}
		if e.Payload.ClientMsgID == "" {
			return errors.New("payload.client_msg_id is required for message.submitted")
		}
		if len(e.Payload.VisibleUserIDs) == 0 {
			return errors.New("payload.visible_user_ids is required for message.submitted")
		}
	}
	return nil
}

func (e MessageEvent) PartitionKey() string {
	return e.ConversationID
}

func (e MessageEvent) Clone() MessageEvent {
	e.Payload = e.Payload.Clone()
	return e
}

func (p MessageEventPayload) Clone() MessageEventPayload {
	if p.ReceiverIDs != nil {
		p.ReceiverIDs = append([]string(nil), p.ReceiverIDs...)
	}
	if p.VisibleUserIDs != nil {
		p.VisibleUserIDs = append([]string(nil), p.VisibleUserIDs...)
	}
	if p.Content != nil {
		p.Content = append(json.RawMessage(nil), p.Content...)
	}
	return p
}

func MarshalMessageEvent(event MessageEvent) ([]byte, error) {
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(event)
}

func UnmarshalMessageEvent(data []byte) (MessageEvent, error) {
	var event MessageEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return MessageEvent{}, err
	}
	if err := event.Validate(); err != nil {
		return MessageEvent{}, err
	}
	return event, nil
}
