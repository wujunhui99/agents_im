package delivery

import "context"

const (
	EventMessageReceived  = "message_received"
	EventMessageDelivered = "message_delivered"
)

const (
	StatusDelivered = "delivered"
	StatusOffline   = "offline"
	StatusFailed    = "failed"
)

type Dispatcher interface {
	DeliverToUser(ctx context.Context, userID string, event Event) (Result, error)
	DeliverToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event Event) (Result, error)
}

type Event struct {
	Type string  `json:"type"`
	Data Message `json:"data"`
}

type Message struct {
	ServerMsgID     string                 `json:"server_msg_id"`
	ClientMsgID     string                 `json:"client_msg_id,omitempty"`
	ConversationID  string                 `json:"conversation_id"`
	Seq             int64                  `json:"seq"`
	SenderID        string                 `json:"sender_id"`
	ReceiverID      string                 `json:"receiver_id,omitempty"`
	GroupID         string                 `json:"group_id,omitempty"`
	ChatType        string                 `json:"chat_type,omitempty"`
	ContentType     string                 `json:"content_type"`
	Content         string                 `json:"content,omitempty"`
	ContentMetadata map[string]interface{} `json:"content_metadata,omitempty"`
	SendTime        int64                  `json:"send_time,omitempty"`
	CreatedAt       int64                  `json:"created_at,omitempty"`
	TraceID         string                 `json:"trace_id,omitempty"`
}

type Result struct {
	ConversationID       string            `json:"conversation_id,omitempty"`
	Recipients           []RecipientResult `json:"recipients"`
	DeliveredRecipients  int               `json:"delivered_recipients"`
	DeliveredConnections int               `json:"delivered_connections"`
	OfflineRecipients    int               `json:"offline_recipients"`
	FailedRecipients     int               `json:"failed_recipients"`
}

type RecipientResult struct {
	UserID                 string   `json:"user_id"`
	Status                 string   `json:"status"`
	DeliveredConnectionIDs []string `json:"delivered_connection_ids,omitempty"`
	FailedConnectionIDs    []string `json:"failed_connection_ids,omitempty"`
	Error                  string   `json:"error,omitempty"`
}

func NewMessageEvent(eventType string, message Message) Event {
	return Event{
		Type: eventType,
		Data: message,
	}
}

func (r *Result) AddRecipient(recipient RecipientResult) {
	r.Recipients = append(r.Recipients, recipient)
	switch recipient.Status {
	case StatusDelivered:
		r.DeliveredRecipients++
		r.DeliveredConnections += len(recipient.DeliveredConnectionIDs)
	case StatusOffline:
		r.OfflineRecipients++
	case StatusFailed:
		r.FailedRecipients++
	}
}
