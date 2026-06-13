package transfergateway

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gatewaydelivery "github.com/wujunhui99/agents_im/common/share/gateway/delivery"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/service/msgtransfer/internal/transfer"
)

var (
	ErrGatewayDispatcherRequired = errors.New("transfer gateway dispatcher: gateway dispatcher is required")
	ErrUnsupportedEventType      = errors.New("transfer gateway dispatcher: unsupported event type")
	ErrNoRecipients              = errors.New("transfer gateway dispatcher: message.accepted has no recipients")
	ErrGatewayDeliveryFailed     = errors.New("transfer gateway dispatcher: gateway delivery failed")
)

type Dispatcher struct {
	gateway    gatewaydelivery.Dispatcher
	retryAfter time.Duration
}

type Option func(*Dispatcher)

func NewDispatcher(dispatcher gatewaydelivery.Dispatcher, opts ...Option) *Dispatcher {
	d := &Dispatcher{gateway: dispatcher}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func WithRetryAfter(retryAfter time.Duration) Option {
	return func(d *Dispatcher) {
		if retryAfter > 0 {
			d.retryAfter = retryAfter
		}
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, envelope transfer.Envelope) transfer.DispatchResult {
	if err := ctx.Err(); err != nil {
		return d.retryable(err)
	}
	if d == nil || d.gateway == nil {
		return d.retryable(ErrGatewayDispatcherRequired)
	}
	if envelope.TraceContext.TraceID != "" {
		ctx = observability.ContextWithTrace(ctx, envelope.TraceContext)
	}
	ctx, span := observability.StartSpan(ctx, "message.transfer.local_gateway_dispatch")
	defer span.End()

	switch strings.TrimSpace(envelope.Event.EventType) {
	case transfer.EventTypeMessageAccepted:
		return d.dispatchMessageAccepted(ctx, envelope)
	default:
		return transfer.DispatchFailed(fmt.Errorf("%w %q", ErrUnsupportedEventType, envelope.Event.EventType))
	}
}

func (d *Dispatcher) dispatchMessageAccepted(ctx context.Context, envelope transfer.Envelope) transfer.DispatchResult {
	recipients := directRecipients(envelope.Event)
	if len(recipients) == 0 {
		return transfer.DispatchFailed(ErrNoRecipients)
	}

	event := messageReceivedEvent(envelope, recipients)
	result, err := d.gateway.DeliverToConversation(ctx, envelope.Event.ConversationID, recipients, event)
	if err != nil {
		dispatch := d.retryable(err)
		dispatch.RecipientResults = failedRecipientResults(recipients, err.Error())
		return dispatch
	}
	if result.FailedRecipients > 0 {
		dispatch := d.retryable(failedDeliveryError(result))
		dispatch.RecipientResults = recipientDeliveryResults(result)
		return dispatch
	}
	if result.RoutedRecipients > 0 {
		dispatch := d.retryable(routedDeliveryError(result))
		dispatch.RecipientResults = recipientDeliveryResults(result)
		return dispatch
	}

	dispatch := transfer.DispatchSucceeded(deliveredUserIDs(result)...)
	dispatch.RecipientResults = recipientDeliveryResults(result)
	return dispatch
}

func (d *Dispatcher) retryable(err error) transfer.DispatchResult {
	retryAfter := time.Duration(0)
	if d != nil {
		retryAfter = d.retryAfter
	}
	return transfer.DispatchRetryable(err, retryAfter)
}

func directRecipients(event transfer.MessageEvent) []string {
	seen := make(map[string]struct{}, len(event.ReceiverIDs)+1)
	recipients := make([]string, 0, len(event.ReceiverIDs)+1)
	for _, userID := range event.ReceiverIDs {
		recipients = appendRecipient(recipients, seen, userID)
	}
	if len(recipients) == 0 {
		recipients = appendRecipient(recipients, seen, event.ReceiverID)
	}
	return recipients
}

func appendRecipient(recipients []string, seen map[string]struct{}, userID string) []string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return recipients
	}
	if _, ok := seen[userID]; ok {
		return recipients
	}
	seen[userID] = struct{}{}
	return append(recipients, userID)
}

func messageReceivedEvent(envelope transfer.Envelope, recipients []string) gatewaydelivery.Event {
	event := envelope.Event
	return gatewaydelivery.NewMessageEvent(gatewaydelivery.EventMessageReceived, gatewaydelivery.Message{
		ServerMsgID:           event.ServerMsgID,
		ClientMsgID:           event.ClientMsgID,
		ConversationID:        event.ConversationID,
		Seq:                   event.Seq,
		SenderID:              event.SenderID,
		ReceiverID:            receiverID(event, recipients),
		GroupID:               event.GroupID,
		ChatType:              event.ChatType,
		ContentType:           event.ContentType,
		Content:               event.Content,
		ContentMetadata:       cloneMetadata(event.ContentMetadata),
		MessageOrigin:         event.MessageOrigin,
		AgentAccountID:        event.AgentAccountID,
		TriggerServerMsgID:    event.TriggerServerMsgID,
		AgentRunID:            event.AgentRunID,
		AllowRecursiveTrigger: event.AllowRecursiveTrigger,
		SendTime:              event.SendTime,
		CreatedAt:             event.CreatedAt,
		TraceID:               event.TraceID,
		RequestID:             event.RequestID,
		TraceParent:           event.TraceParent,
		TraceState:            event.TraceState,
	})
}

func receiverID(event transfer.MessageEvent, recipients []string) string {
	if receiverID := strings.TrimSpace(event.ReceiverID); receiverID != "" {
		return receiverID
	}
	if len(recipients) == 1 {
		return recipients[0]
	}
	return ""
}

func cloneMetadata(metadata map[string]interface{}) map[string]interface{} {
	if metadata == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(metadata))
	for key, value := range metadata {
		clone[key] = value
	}
	return clone
}

func deliveredUserIDs(result gatewaydelivery.Result) []string {
	delivered := make([]string, 0, result.DeliveredRecipients)
	for _, recipient := range result.Recipients {
		if recipient.Status == gatewaydelivery.StatusDelivered {
			delivered = append(delivered, recipient.UserID)
		}
	}
	return delivered
}

func recipientDeliveryResults(result gatewaydelivery.Result) []transfer.RecipientDeliveryResult {
	recipients := make([]transfer.RecipientDeliveryResult, 0, len(result.Recipients))
	for _, recipient := range result.Recipients {
		recipients = append(recipients, transfer.RecipientDeliveryResult{
			UserID: recipient.UserID,
			Status: transferRecipientStatus(recipient.Status),
			Error:  transferRecipientError(recipient),
		})
	}
	return recipients
}

func transferRecipientStatus(status string) transfer.RecipientDeliveryStatus {
	switch status {
	case gatewaydelivery.StatusDelivered:
		return transfer.RecipientDeliveryDelivered
	case gatewaydelivery.StatusOffline:
		return transfer.RecipientDeliveryOffline
	default:
		return transfer.RecipientDeliveryFailed
	}
}

func transferRecipientError(recipient gatewaydelivery.RecipientResult) string {
	if strings.TrimSpace(recipient.Error) != "" {
		return recipient.Error
	}
	if recipient.Status == gatewaydelivery.StatusRouted {
		return "recipient routed to remote gateway; remote delivery is not implemented"
	}
	if recipient.Status == gatewaydelivery.StatusFailed {
		return "gateway delivery failed"
	}
	return ""
}

func failedRecipientResults(userIDs []string, message string) []transfer.RecipientDeliveryResult {
	results := make([]transfer.RecipientDeliveryResult, 0, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		results = append(results, transfer.RecipientDeliveryResult{
			UserID: userID,
			Status: transfer.RecipientDeliveryFailed,
			Error:  strings.TrimSpace(message),
		})
	}
	return results
}

func failedDeliveryError(result gatewaydelivery.Result) error {
	failures := make([]string, 0, result.FailedRecipients)
	for _, recipient := range result.Recipients {
		if recipient.Status != gatewaydelivery.StatusFailed {
			continue
		}
		if strings.TrimSpace(recipient.Error) == "" {
			failures = append(failures, recipient.UserID)
			continue
		}
		failures = append(failures, recipient.UserID+": "+recipient.Error)
	}
	if len(failures) == 0 {
		return ErrGatewayDeliveryFailed
	}
	return fmt.Errorf("%w: %s", ErrGatewayDeliveryFailed, strings.Join(failures, ", "))
}

func routedDeliveryError(result gatewaydelivery.Result) error {
	routed := make([]string, 0, result.RoutedRecipients)
	for _, recipient := range result.Recipients {
		if recipient.Status == gatewaydelivery.StatusRouted {
			routed = append(routed, recipient.UserID)
		}
	}
	if len(routed) == 0 {
		return ErrGatewayDeliveryFailed
	}
	return fmt.Errorf("%w: routed recipients require remote gateway delivery: %s", ErrGatewayDeliveryFailed, strings.Join(routed, ", "))
}
