package transfer

import "time"

const (
	EventTypeMessageAccepted = "message.accepted"
)

type MessageEvent struct {
	EventID        string   `json:"eventId"`
	EventType      string   `json:"eventType"`
	ConversationID string   `json:"conversationId"`
	Seq            int64    `json:"seq"`
	ServerMsgID    string   `json:"serverMsgId"`
	SenderID       string   `json:"senderId"`
	ReceiverIDs    []string `json:"receiverIds"`
	CreatedAt      int64    `json:"createdAt"`
	TraceID        string   `json:"traceId,omitempty"`
}

type Envelope struct {
	ID         string
	Topic      string
	Key        string
	Partition  int32
	Offset     int64
	Attempt    int
	ReceivedAt time.Time
	Event      MessageEvent
	RawPayload []byte
}

func (e Envelope) IdempotencyKey() string {
	if e.Event.EventID != "" {
		return e.Event.EventID
	}
	if e.ID != "" {
		return e.ID
	}
	if e.Event.ServerMsgID != "" {
		return e.Event.ServerMsgID
	}
	return e.Key
}
