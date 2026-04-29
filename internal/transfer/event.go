package transfer

import "time"

const (
	EventTypeMessageAccepted = "message.accepted"
)

type MessageEvent struct {
	EventID         string                 `json:"eventId"`
	EventType       string                 `json:"eventType"`
	ConversationID  string                 `json:"conversationId"`
	Seq             int64                  `json:"seq"`
	ServerMsgID     string                 `json:"serverMsgId"`
	SenderID        string                 `json:"senderId"`
	ReceiverID      string                 `json:"receiverId,omitempty"`
	ReceiverIDs     []string               `json:"receiverIds"`
	GroupID         string                 `json:"groupId,omitempty"`
	ChatType        string                 `json:"chatType,omitempty"`
	ClientMsgID     string                 `json:"clientMsgId,omitempty"`
	ContentType     string                 `json:"contentType,omitempty"`
	Content         string                 `json:"content,omitempty"`
	ContentMetadata map[string]interface{} `json:"contentMetadata,omitempty"`
	SendTime        int64                  `json:"sendTime,omitempty"`
	CreatedAt       int64                  `json:"createdAt"`
	TraceID         string                 `json:"traceId,omitempty"`
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
