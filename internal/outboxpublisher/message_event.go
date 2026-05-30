package outboxpublisher

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/messaging"

	"github.com/wujunhui99/agents_im/internal/repository"
)

// MessageEventFromOutbox converts a durable message_outbox row into the
// messaging.MessageEvent consumed by the message-transfer outbox worker. This is
// the live V1 fanout path; the former Kafka publisher that also used it has been
// removed (Redpanda/Kafka were unused).
func MessageEventFromOutbox(event repository.OutboxEvent) (messaging.MessageEvent, error) {
	if event.EventType != repository.OutboxEventTypeMessageCreated {
		return messaging.MessageEvent{}, fmt.Errorf("unsupported outbox event_type %q", event.EventType)
	}
	if event.AggregateType != 0 && event.AggregateType != repository.OutboxAggregateTypeMessage {
		return messaging.MessageEvent{}, fmt.Errorf("unsupported outbox aggregate_type %q", event.AggregateType)
	}

	var payload repository.MessageCreatedOutboxPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return messaging.MessageEvent{}, fmt.Errorf("decode message.created outbox payload: %w", err)
	}

	message := payload.Message
	content, err := messageContent(message)
	if err != nil {
		return messaging.MessageEvent{}, err
	}
	accepted := messaging.MessageEvent{
		EventID:        event.EventID,
		EventType:      messaging.EventTypeMessageAccepted,
		ConversationID: firstNonEmpty(message.ConversationID, event.ConversationID),
		ServerMsgID:    firstNonEmpty(message.ServerMsgID, event.ServerMsgID),
		Seq:            firstNonZero(message.Seq, event.Seq),
		SenderID:       message.SenderID,
		ChatType:       message.ChatType,
		CreatedAt:      messageCreatedAt(message, event),
		Payload: messaging.MessageEventPayload{
			ClientMsgID:           message.ClientMsgID,
			ReceiverID:            message.ReceiverID,
			ReceiverIDs:           receiverIDs(message, payload.VisibleUserIDs),
			GroupID:               message.GroupID,
			ContentType:           message.ContentType,
			Content:               content,
			MessageOrigin:         message.MessageOrigin,
			AgentAccountID:        message.AgentAccountID,
			TriggerServerMsgID:    message.TriggerServerMsgID,
			AgentRunID:            message.AgentRunID,
			AllowRecursiveTrigger: message.AllowRecursiveTrigger,
			TraceID:               payload.TraceContext.TraceID,
			RequestID:             payload.TraceContext.RequestID,
			TraceParent:           payload.TraceContext.TraceParent,
			TraceState:            payload.TraceContext.TraceState,
		},
	}
	if err := accepted.Validate(); err != nil {
		return messaging.MessageEvent{}, fmt.Errorf("build message.accepted event: %w", err)
	}
	return accepted, nil
}

func messageContent(message repository.Message) (json.RawMessage, error) {
	switch message.ContentType {
	case "", repository.ContentTypeText:
		content, err := json.Marshal(map[string]string{"text": message.Content})
		if err != nil {
			return nil, fmt.Errorf("encode text message content: %w", err)
		}
		return json.RawMessage(content), nil
	default:
		content := strings.TrimSpace(message.Content)
		if content == "" {
			return nil, nil
		}
		if json.Valid([]byte(content)) {
			return json.RawMessage(append([]byte(nil), content...)), nil
		}
		encoded, err := json.Marshal(message.Content)
		if err != nil {
			return nil, fmt.Errorf("encode message content: %w", err)
		}
		return json.RawMessage(encoded), nil
	}
}

func receiverIDs(message repository.Message, visibleUserIDs []string) []string {
	ids := make([]string, 0, len(visibleUserIDs)+1)
	if message.ChatType == repository.ChatTypeSingle && message.ReceiverID != "" {
		ids = append(ids, message.ReceiverID)
	}
	includeSender := shouldDeliverMessageToSender(message)
	if includeSender {
		ids = append(ids, message.SenderID)
	}
	for _, userID := range visibleUserIDs {
		if userID != "" && (includeSender || userID != message.SenderID) {
			ids = append(ids, userID)
		}
	}
	return uniqueSorted(ids)
}

func shouldDeliverMessageToSender(message repository.Message) bool {
	return strings.ToLower(strings.TrimSpace(message.ChatType)) == repository.ChatTypeSingle &&
		strings.ToLower(strings.TrimSpace(message.MessageOrigin)) == repository.MessageOriginAI
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	unique := values[:0]
	var previous string
	for _, value := range values {
		if value == "" || value == previous {
			continue
		}
		unique = append(unique, value)
		previous = value
	}
	return append([]string(nil), unique...)
}

func messageCreatedAt(message repository.Message, event repository.OutboxEvent) int64 {
	if message.CreatedAt > 0 {
		return message.CreatedAt
	}
	if message.SendTime > 0 {
		return message.SendTime
	}
	if !event.CreatedAt.IsZero() {
		return event.CreatedAt.UTC().UnixMilli()
	}
	return 0
}

func firstNonEmpty(first string, second string) string {
	if first != "" {
		return first
	}
	return second
}

func firstNonZero(first int64, second int64) int64 {
	if first != 0 {
		return first
	}
	return second
}
