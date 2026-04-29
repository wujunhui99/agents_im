package repository

import (
	"context"
	"sort"
	"strings"
	"time"
)

var _ DeliveryAttemptRepository = (*MemoryMessageRepository)(nil)

func (r *MemoryMessageRepository) CreateDeliveryAttemptsAccepted(_ context.Context, attempts []CreateDeliveryAttemptInput) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.createDeliveryAttemptsAcceptedLocked(attempts, r.now().UTC())
}

func (r *MemoryMessageRepository) MarkDeliveryAttemptsPublished(_ context.Context, serverMsgID string, recipientUserIDs []string) error {
	serverMsgID = strings.TrimSpace(serverMsgID)
	if serverMsgID == "" {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.markDeliveryAttemptsPublishedLocked(serverMsgID, recipientUserIDs, r.now().UTC())
	return nil
}

func (r *MemoryMessageRepository) RecordDeliveryAttemptResult(_ context.Context, input RecordDeliveryAttemptInput) error {
	normalized, err := normalizeRecordDeliveryAttemptInput(input)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now().UTC()
	key := deliveryAttemptKey(normalized.ServerMsgID, normalized.RecipientUserID)
	attempt, exists := r.deliveryAttempts[key]
	if !exists {
		attempt = DeliveryAttempt{
			ServerMsgID:     normalized.ServerMsgID,
			ConversationID:  normalized.ConversationID,
			RecipientUserID: normalized.RecipientUserID,
			CreatedAt:       now,
		}
	}
	nextAttemptCount := attempt.AttemptCount + 1
	if normalized.AttemptCount > nextAttemptCount {
		nextAttemptCount = normalized.AttemptCount
	}
	attempt.ConversationID = normalized.ConversationID
	attempt.Status = normalized.Status
	attempt.AttemptCount = nextAttemptCount
	attempt.LastError = normalized.LastError
	attempt.NextRetryAt = normalized.NextRetryAt
	attempt.UpdatedAt = now
	r.deliveryAttempts[key] = attempt
	return nil
}

func (r *MemoryMessageRepository) ListDeliveryAttemptsByMessage(_ context.Context, serverMsgID string) ([]DeliveryAttempt, error) {
	serverMsgID = strings.TrimSpace(serverMsgID)
	if serverMsgID == "" {
		return nil, nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	attempts := make([]DeliveryAttempt, 0)
	for _, attempt := range r.deliveryAttempts {
		if attempt.ServerMsgID == serverMsgID {
			attempts = append(attempts, attempt.Clone())
		}
	}
	sort.Slice(attempts, func(i, j int) bool {
		return attempts[i].RecipientUserID < attempts[j].RecipientUserID
	})
	return attempts, nil
}

func (r *MemoryMessageRepository) createDeliveryAttemptsAcceptedLocked(attempts []CreateDeliveryAttemptInput, now time.Time) error {
	for _, attemptInput := range attempts {
		normalized, err := normalizeCreateDeliveryAttemptInput(attemptInput)
		if err != nil {
			return err
		}
		key := deliveryAttemptKey(normalized.ServerMsgID, normalized.RecipientUserID)
		if _, exists := r.deliveryAttempts[key]; exists {
			continue
		}
		r.deliveryAttempts[key] = DeliveryAttempt{
			ServerMsgID:     normalized.ServerMsgID,
			ConversationID:  normalized.ConversationID,
			RecipientUserID: normalized.RecipientUserID,
			Status:          DeliveryStatusAccepted,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
	}
	return nil
}

func (r *MemoryMessageRepository) markDeliveryAttemptsPublishedLocked(serverMsgID string, recipientUserIDs []string, now time.Time) {
	recipients := deliveryRecipientSet(recipientUserIDs)
	for key, attempt := range r.deliveryAttempts {
		if attempt.ServerMsgID != serverMsgID {
			continue
		}
		if len(recipients) > 0 {
			if _, ok := recipients[attempt.RecipientUserID]; !ok {
				continue
			}
		}
		if attempt.Status != DeliveryStatusAccepted && attempt.Status != DeliveryStatusPublished {
			continue
		}
		attempt.Status = DeliveryStatusPublished
		attempt.UpdatedAt = now
		r.deliveryAttempts[key] = attempt
	}
}

func deliveryAttemptsForMessage(message Message, input CreateMessageInput) []CreateDeliveryAttemptInput {
	recipients := deliveryAttemptRecipientUserIDs(input)
	attempts := make([]CreateDeliveryAttemptInput, 0, len(recipients))
	for _, recipientUserID := range recipients {
		attempts = append(attempts, CreateDeliveryAttemptInput{
			ServerMsgID:     message.ServerMsgID,
			ConversationID:  message.ConversationID,
			RecipientUserID: recipientUserID,
		})
	}
	return attempts
}

func deliveryRecipientSet(recipientUserIDs []string) map[string]struct{} {
	if len(recipientUserIDs) == 0 {
		return nil
	}
	recipients := make(map[string]struct{}, len(recipientUserIDs))
	for _, userID := range recipientUserIDs {
		userID = strings.TrimSpace(userID)
		if userID != "" {
			recipients[userID] = struct{}{}
		}
	}
	return recipients
}

func deliveryAttemptKey(serverMsgID string, recipientUserID string) string {
	return strings.TrimSpace(serverMsgID) + "\x00" + strings.TrimSpace(recipientUserID)
}
