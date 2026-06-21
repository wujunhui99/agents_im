package pusher

import (
	"sort"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/gateway/delivery"
	"github.com/wujunhui99/agents_im/pkg/messaging"
	"github.com/wujunhui99/agents_im/pkg/observability"
)

// pushRecipients are the users a toPush event fans out to. msgtransfer already
// resolved the full participant set (sender + receiver + visible members, agents
// filtered per D16) into payload.receiver_ids (03 §5.2), so push does NOT reload
// group membership — it only delivers. Falls back to the single receiver_id.
func pushRecipients(event messaging.MessageEvent) []string {
	ids := make([]string, 0, len(event.Payload.ReceiverIDs)+1)
	ids = append(ids, event.Payload.ReceiverIDs...)
	if len(ids) == 0 {
		ids = append(ids, event.Payload.ReceiverID)
	}
	return uniqueNonEmpty(ids)
}

// deliveryEventFromMessaging maps a toPush message.accepted event to the gateway
// message_received delivery event (mirrors the retired transfer dispatcher).
func deliveryEventFromMessaging(event messaging.MessageEvent) delivery.Event {
	return delivery.NewMessageEvent(delivery.EventMessageReceived, delivery.Message{
		ServerMsgID:           event.ServerMsgID,
		ClientMsgID:           event.Payload.ClientMsgID,
		ConversationID:        event.ConversationID,
		Seq:                   event.Seq,
		SenderID:              event.SenderID,
		ReceiverID:            event.Payload.ReceiverID,
		GroupID:               event.Payload.GroupID,
		ChatType:              event.ChatType,
		ContentType:           event.Payload.ContentType,
		Content:               string(event.Payload.Content),
		MessageOrigin:         event.Payload.MessageOrigin,
		AgentAccountID:        event.Payload.AgentAccountID,
		TriggerServerMsgID:    event.Payload.TriggerServerMsgID,
		AgentRunID:            event.Payload.AgentRunID,
		AllowRecursiveTrigger: event.Payload.AllowRecursiveTrigger,
		SendTime:              event.Payload.SendTime,
		CreatedAt:             event.CreatedAt,
		TraceID:               event.Payload.TraceID,
		RequestID:             event.Payload.RequestID,
		TraceParent:           event.Payload.TraceParent,
		TraceState:            event.Payload.TraceState,
	})
}

// traceContext extracts the propagation context carried in the event payload so
// push spans + gateway gRPC calls join the originating message trace.
func traceContext(event messaging.MessageEvent) observability.TraceContext {
	if strings.TrimSpace(event.Payload.TraceID) == "" {
		return observability.TraceContext{}
	}
	tc := observability.NewTraceContext(event.Payload.TraceID, event.Payload.RequestID)
	tc.TraceParent = event.Payload.TraceParent
	tc.TraceState = event.Payload.TraceState
	return tc
}

func uniqueNonEmpty(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	sort.Strings(cleaned)
	return cleaned
}
