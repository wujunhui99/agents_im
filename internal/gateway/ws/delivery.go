package ws

import (
	"context"
	"errors"
	"strings"

	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/pkg/observability"
	"github.com/wujunhui99/agents_im/pkg/presence"
)

var errNilConnectionManager = errors.New("connection manager is not configured")

type InMemoryDeliveryDispatcher struct {
	connections *ConnectionManager
	presence    presence.PresenceStore
	instanceID  string
}

func NewInMemoryDeliveryDispatcher(manager *ConnectionManager) *InMemoryDeliveryDispatcher {
	return &InMemoryDeliveryDispatcher{connections: manager}
}

func NewPresenceAwareDeliveryDispatcher(manager *ConnectionManager, store presence.PresenceStore, instanceID string) *InMemoryDeliveryDispatcher {
	return &InMemoryDeliveryDispatcher{
		connections: manager,
		presence:    store,
		instanceID:  strings.TrimSpace(instanceID),
	}
}

func (d *InMemoryDeliveryDispatcher) DeliverToUser(ctx context.Context, userID string, event delivery.Event) (delivery.Result, error) {
	ctx, span := observability.StartSpan(ctx, "gateway.delivery.user")
	defer span.End()
	if d == nil || d.connections == nil {
		observability.RecordSpanError(span, errNilConnectionManager)
		return delivery.Result{}, errNilConnectionManager
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return delivery.Result{}, apperror.InvalidArgument("delivery user_id is required")
	}
	if strings.TrimSpace(event.Type) == "" {
		return delivery.Result{}, apperror.InvalidArgument("delivery event type is required")
	}

	result := delivery.Result{ConversationID: event.Data.ConversationID}
	recipient, err := d.deliverToUser(ctx, userID, event)
	result.AddRecipient(recipient)
	return result, err
}

func (d *InMemoryDeliveryDispatcher) DeliverToConversation(ctx context.Context, conversationID string, recipientUserIDs []string, event delivery.Event) (delivery.Result, error) {
	if event.Data.TraceID != "" {
		ctx = observability.ContextWithTrace(ctx, observability.TraceContext{
			TraceID:     event.Data.TraceID,
			RequestID:   event.Data.RequestID,
			TraceParent: event.Data.TraceParent,
			TraceState:  event.Data.TraceState,
		})
	}
	ctx, span := observability.StartSpan(ctx, "gateway.delivery.conversation")
	defer span.End()
	if d == nil || d.connections == nil {
		observability.RecordSpanError(span, errNilConnectionManager)
		return delivery.Result{}, errNilConnectionManager
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return delivery.Result{}, apperror.InvalidArgument("conversation_id is required")
	}
	if strings.TrimSpace(event.Type) == "" {
		return delivery.Result{}, apperror.InvalidArgument("delivery event type is required")
	}
	if strings.TrimSpace(event.Data.ConversationID) == "" {
		event.Data.ConversationID = conversationID
	}

	result := delivery.Result{ConversationID: conversationID}
	seen := make(map[string]struct{}, len(recipientUserIDs))
	for _, recipientUserID := range recipientUserIDs {
		recipientUserID = strings.TrimSpace(recipientUserID)
		if recipientUserID == "" {
			continue
		}
		if _, ok := seen[recipientUserID]; ok {
			continue
		}
		seen[recipientUserID] = struct{}{}

		recipient, err := d.deliverToUser(ctx, recipientUserID, event)
		result.AddRecipient(recipient)
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (d *InMemoryDeliveryDispatcher) deliverToUser(ctx context.Context, userID string, event delivery.Event) (delivery.RecipientResult, error) {
	record := func(recipient delivery.RecipientResult, err error) (delivery.RecipientResult, error) {
		if recipient.Status != "" {
			observability.RecordDeliveryAttempt(string(recipient.Status))
		}
		return recipient, err
	}

	if err := ctx.Err(); err != nil {
		return record(delivery.RecipientResult{
			UserID: userID,
			Status: delivery.StatusFailed,
			Error:  err.Error(),
		}, err)
	}

	routes, err := d.lookupRoutes(ctx, userID)
	if err != nil {
		return record(delivery.RecipientResult{
			UserID: userID,
			Status: delivery.StatusFailed,
			Error:  err.Error(),
		}, err)
	}
	if d.presence != nil && len(routes) == 0 {
		return record(delivery.RecipientResult{
			UserID: userID,
			Status: delivery.StatusOffline,
		}, nil)
	}

	connections := d.connections.UserConnections(userID)
	if len(connections) == 0 {
		if len(routes) > 0 {
			return record(delivery.RecipientResult{
				UserID: userID,
				Status: delivery.StatusRouted,
				Routes: routes,
			}, nil)
		}
		return record(delivery.RecipientResult{
			UserID: userID,
			Status: delivery.StatusOffline,
		}, nil)
	}

	recipient := delivery.RecipientResult{
		UserID: userID,
		Status: delivery.StatusFailed,
		Routes: routes,
	}
	for _, conn := range connections {
		if err := ctx.Err(); err != nil {
			recipient.Error = err.Error()
			return record(recipient, err)
		}
		if conn == nil {
			continue
		}
		if err := conn.writeJSON(event); err != nil {
			recipient.FailedConnectionIDs = append(recipient.FailedConnectionIDs, conn.ID)
			recipient.Error = err.Error()
			d.connections.Unregister(conn.ID)
			continue
		}
		recipient.DeliveredConnectionIDs = append(recipient.DeliveredConnectionIDs, conn.ID)
	}
	if len(recipient.DeliveredConnectionIDs) > 0 {
		recipient.Status = delivery.StatusDelivered
	}
	if len(recipient.FailedConnectionIDs) == 0 {
		recipient.Error = ""
	}
	return record(recipient, nil)
}

func (d *InMemoryDeliveryDispatcher) lookupRoutes(ctx context.Context, userID string) ([]delivery.Route, error) {
	if d.presence == nil {
		return nil, nil
	}
	connections, err := d.presence.ListUserConnections(ctx, userID)
	if err != nil {
		return nil, err
	}
	routes := make([]delivery.Route, 0, len(connections))
	for _, metadata := range connections {
		routes = append(routes, delivery.Route{
			UserID:       metadata.UserID,
			ConnectionID: metadata.ConnectionID,
			InstanceID:   metadata.InstanceID,
			GatewayID:    metadata.GatewayID,
			DeviceID:     metadata.DeviceID,
			Platform:     metadata.Platform,
			Local:        sameInstance(metadata.InstanceID, metadata.GatewayID, d.instanceID),
		})
	}
	return routes, nil
}

func sameInstance(instanceID string, gatewayID string, localInstanceID string) bool {
	localInstanceID = strings.TrimSpace(localInstanceID)
	if localInstanceID == "" {
		return false
	}
	return strings.TrimSpace(instanceID) == localInstanceID || strings.TrimSpace(gatewayID) == localInstanceID
}
