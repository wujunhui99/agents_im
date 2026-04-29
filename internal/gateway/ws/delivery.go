package ws

import (
	"context"
	"errors"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/gateway/delivery"
)

var errNilConnectionManager = errors.New("connection manager is not configured")

type InMemoryDeliveryDispatcher struct {
	connections *ConnectionManager
}

func NewInMemoryDeliveryDispatcher(manager *ConnectionManager) *InMemoryDeliveryDispatcher {
	return &InMemoryDeliveryDispatcher{connections: manager}
}

func (d *InMemoryDeliveryDispatcher) DeliverToUser(ctx context.Context, userID string, event delivery.Event) (delivery.Result, error) {
	if d == nil || d.connections == nil {
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
	if d == nil || d.connections == nil {
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
	if err := ctx.Err(); err != nil {
		return delivery.RecipientResult{
			UserID: userID,
			Status: delivery.StatusFailed,
			Error:  err.Error(),
		}, err
	}

	connections := d.connections.UserConnections(userID)
	if len(connections) == 0 {
		return delivery.RecipientResult{
			UserID: userID,
			Status: delivery.StatusOffline,
		}, nil
	}

	recipient := delivery.RecipientResult{
		UserID: userID,
		Status: delivery.StatusFailed,
	}
	for _, conn := range connections {
		if err := ctx.Err(); err != nil {
			recipient.Error = err.Error()
			return recipient, err
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
	return recipient, nil
}
