package repository

import "strings"

// DeliveryRecipientUserIDs returns the intended live-push recipients for a
// message. V2 no longer persists per-recipient delivery rows; this helper is
// only for immediate push hooks and outbox payload construction.
func DeliveryRecipientUserIDs(input CreateMessageInput) []string {
	seen := make(map[string]struct{})
	add := func(userID string) {
		userID = strings.TrimSpace(userID)
		if userID == "" || userID == input.SenderID {
			return
		}
		seen[userID] = struct{}{}
	}
	for _, userID := range input.ParticipantUserIDs {
		add(userID)
	}
	if input.ChatType == ChatTypeSingle {
		add(input.ReceiverID)
	}
	users := make([]string, 0, len(seen))
	for userID := range seen {
		users = append(users, userID)
	}
	return users
}
