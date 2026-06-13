package transfer

import (
	"strings"

	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
)

// MessageEventFromMessagingEvent adapts a messaging.MessageEvent (produced from a
// durable outbox row) into the transfer worker's MessageEvent. It is part of the
// live outbox fanout path; the former Kafka consumer that also produced these has
// been removed.
func MessageEventFromMessagingEvent(event messaging.MessageEvent) MessageEvent {
	return MessageEvent{
		EventID:               event.EventID,
		EventType:             EventTypeMessageAccepted,
		ConversationID:        event.ConversationID,
		Seq:                   event.Seq,
		ServerMsgID:           event.ServerMsgID,
		SenderID:              event.SenderID,
		ReceiverIDs:           receiverIDsFromMessagingEvent(event),
		ReceiverID:            event.Payload.ReceiverID,
		GroupID:               event.Payload.GroupID,
		ChatType:              event.ChatType,
		ClientMsgID:           event.Payload.ClientMsgID,
		ContentType:           event.Payload.ContentType,
		Content:               string(event.Payload.Content),
		MessageOrigin:         event.Payload.MessageOrigin,
		AgentAccountID:        event.Payload.AgentAccountID,
		TriggerServerMsgID:    event.Payload.TriggerServerMsgID,
		AgentRunID:            event.Payload.AgentRunID,
		AllowRecursiveTrigger: event.Payload.AllowRecursiveTrigger,
		CreatedAt:             event.CreatedAt,
		TraceID:               event.Payload.TraceID,
		RequestID:             event.Payload.RequestID,
		TraceParent:           event.Payload.TraceParent,
		TraceState:            event.Payload.TraceState,
	}
}

func traceContextFromMessagingEvent(event messaging.MessageEvent) observability.TraceContext {
	traceContext := observability.NewTraceContext(event.Payload.TraceID, event.Payload.RequestID)
	traceContext.TraceParent = event.Payload.TraceParent
	traceContext.TraceState = event.Payload.TraceState
	if event.Payload.TraceID == "" {
		return observability.TraceContext{}
	}
	return traceContext
}

func receiverIDsFromMessagingEvent(event messaging.MessageEvent) []string {
	receiverIDs := make([]string, 0, len(event.Payload.ReceiverIDs)+1)
	seen := make(map[string]struct{}, len(event.Payload.ReceiverIDs)+1)
	for _, receiverID := range event.Payload.ReceiverIDs {
		receiverID = strings.TrimSpace(receiverID)
		if receiverID == "" {
			continue
		}
		if _, ok := seen[receiverID]; ok {
			continue
		}
		seen[receiverID] = struct{}{}
		receiverIDs = append(receiverIDs, receiverID)
	}
	receiverID := strings.TrimSpace(event.Payload.ReceiverID)
	if receiverID != "" {
		if _, ok := seen[receiverID]; !ok {
			receiverIDs = append(receiverIDs, receiverID)
		}
	}
	return receiverIDs
}
