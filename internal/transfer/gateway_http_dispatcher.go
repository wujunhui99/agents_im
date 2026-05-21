package transfer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/propagation"

	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/internal/observability"
)

const defaultGatewayHTTPDispatcherTimeout = 5 * time.Second

type GatewayHTTPDispatcherConfig struct {
	Endpoint string
	Timeout  time.Duration
	Client   *http.Client
}

type GatewayHTTPDispatcher struct {
	endpoint string
	client   *http.Client
}

type gatewayConversationDeliveryRequest struct {
	ConversationID   string         `json:"conversation_id"`
	RecipientUserIDs []string       `json:"recipient_user_ids"`
	Event            delivery.Event `json:"event"`
}

func NewGatewayHTTPDispatcher(cfg GatewayHTTPDispatcherConfig) *GatewayHTTPDispatcher {
	client := cfg.Client
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = defaultGatewayHTTPDispatcherTimeout
		}
		client = &http.Client{Timeout: timeout}
	}
	return &GatewayHTTPDispatcher{
		endpoint: strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/"),
		client:   client,
	}
}

func (d *GatewayHTTPDispatcher) Dispatch(ctx context.Context, envelope Envelope) DispatchResult {
	if envelope.TraceContext.TraceID != "" {
		ctx = observability.ContextWithTrace(ctx, envelope.TraceContext)
	}
	ctx, span := observability.StartSpan(ctx, "message.transfer.gateway_dispatch")
	defer span.End()
	if err := ctx.Err(); err != nil {
		observability.RecordSpanError(span, err)
		return DispatchRetryable(err, 0)
	}
	if d == nil || strings.TrimSpace(d.endpoint) == "" {
		err := errors.New("gateway dispatcher endpoint is required")
		observability.RecordSpanError(span, err)
		return DispatchFailed(err)
	}
	if len(envelope.Event.ReceiverIDs) == 0 && strings.TrimSpace(envelope.Event.ReceiverID) != "" {
		envelope.Event.ReceiverIDs = []string{strings.TrimSpace(envelope.Event.ReceiverID)}
	}
	if len(envelope.Event.ReceiverIDs) == 0 {
		return DispatchSucceeded()
	}

	requestPayload := gatewayConversationDeliveryRequest{
		ConversationID:   envelope.Event.ConversationID,
		RecipientUserIDs: cleanUserIDs(envelope.Event.ReceiverIDs),
		Event:            delivery.NewMessageEvent(delivery.EventMessageReceived, deliveryMessageFromTransferEvent(envelope.Event)),
	}
	if len(requestPayload.RecipientUserIDs) == 0 {
		return DispatchSucceeded()
	}
	if strings.TrimSpace(requestPayload.ConversationID) == "" {
		requestPayload.ConversationID = requestPayload.Event.Data.ConversationID
	}

	body, err := json.Marshal(requestPayload)
	if err != nil {
		return DispatchFailed(err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint+"/internal/delivery/conversation", bytes.NewReader(body))
	if err != nil {
		observability.RecordSpanError(span, err)
		return DispatchFailed(err)
	}
	request.Header.Set("Content-Type", "application/json")
	observability.InjectTraceContext(ctx, propagation.HeaderCarrier(request.Header))

	client := d.client
	if client == nil {
		client = &http.Client{Timeout: defaultGatewayHTTPDispatcherTimeout}
	}
	response, err := client.Do(request)
	if err != nil {
		observability.RecordSpanError(span, err)
		return DispatchRetryable(err, 0)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode >= http.StatusInternalServerError || response.StatusCode == http.StatusTooManyRequests {
		err := fmt.Errorf("gateway dispatcher status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
		observability.RecordSpanError(span, err)
		return DispatchRetryable(err, 0)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("gateway dispatcher status %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
		observability.RecordSpanError(span, err)
		return DispatchFailed(err)
	}

	var deliveryResult delivery.Result
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &deliveryResult); err != nil {
			observability.RecordSpanError(span, err)
			return DispatchRetryable(err, 0)
		}
	}
	return dispatchResultFromGateway(deliveryResult, requestPayload.RecipientUserIDs)
}

func deliveryMessageFromTransferEvent(event MessageEvent) delivery.Message {
	return delivery.Message{
		ServerMsgID:           event.ServerMsgID,
		ClientMsgID:           event.ClientMsgID,
		ConversationID:        event.ConversationID,
		Seq:                   event.Seq,
		SenderID:              event.SenderID,
		ReceiverID:            event.ReceiverID,
		GroupID:               event.GroupID,
		ChatType:              event.ChatType,
		ContentType:           event.ContentType,
		Content:               event.Content,
		ContentMetadata:       event.ContentMetadata,
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
	}
}

func dispatchResultFromGateway(result delivery.Result, fallbackRecipients []string) DispatchResult {
	transferResult := DispatchResult{Status: StatusSucceeded}
	if len(result.Recipients) == 0 {
		transferResult.DeliveredUserIDs = append([]string(nil), fallbackRecipients...)
		return transferResult
	}
	for _, recipient := range result.Recipients {
		switch recipient.Status {
		case delivery.StatusDelivered:
			transferResult.DeliveredUserIDs = append(transferResult.DeliveredUserIDs, recipient.UserID)
			transferResult.RecipientResults = append(transferResult.RecipientResults, RecipientDeliveryResult{UserID: recipient.UserID, Status: RecipientDeliveryDelivered})
		case delivery.StatusOffline, delivery.StatusRouted:
			transferResult.RecipientResults = append(transferResult.RecipientResults, RecipientDeliveryResult{UserID: recipient.UserID, Status: RecipientDeliveryOffline})
		case delivery.StatusFailed:
			transferResult.RecipientResults = append(transferResult.RecipientResults, RecipientDeliveryResult{UserID: recipient.UserID, Status: RecipientDeliveryFailed, Error: recipient.Error})
			transferResult.Status = StatusRetryable
			transferResult.Retryable = true
			if recipient.Error != "" {
				transferResult.Err = errors.New(recipient.Error)
			}
		default:
			transferResult.RecipientResults = append(transferResult.RecipientResults, RecipientDeliveryResult{UserID: recipient.UserID, Status: RecipientDeliveryOffline})
		}
	}
	if transferResult.Status == StatusRetryable {
		if transferResult.Err == nil {
			transferResult.Err = errors.New("gateway delivery failed")
		}
		return transferResult
	}
	transferResult.Status = StatusSucceeded
	return transferResult
}

func cleanUserIDs(userIDs []string) []string {
	seen := make(map[string]struct{}, len(userIDs))
	cleaned := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		cleaned = append(cleaned, userID)
	}
	return cleaned
}
