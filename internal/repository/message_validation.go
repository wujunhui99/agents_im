package repository

import (
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

const (
	defaultMessagePullLimit        = 50
	maxMessagePullLimit            = 500
	maxMessageIDLength             = 128
	maxMessageContentLength        = 4096
	maxMessageConversationIDLength = 256
	messageConversationIDDelimiter = ":"
	messageStorageIdempotencyDelim = "\x00"
)

func validateCreateMessageInput(input CreateMessageInput) (string, error) {
	if err := validateMessageConversationComponentID(input.SenderID, "sender_id"); err != nil {
		return "", err
	}
	if err := validateMessageRequiredID(input.ClientMsgID, "client_msg_id"); err != nil {
		return "", err
	}
	if input.ContentType != ContentTypeText {
		return "", apperror.InvalidArgument("content_type must be text")
	}
	if strings.TrimSpace(input.Content) == "" {
		return "", apperror.InvalidArgument("content is required")
	}
	if len([]rune(input.Content)) > maxMessageContentLength {
		return "", apperror.InvalidArgument("content must be 4096 characters or fewer")
	}
	for _, userID := range input.ParticipantUserIDs {
		if userID == "" {
			continue
		}
		if err := validateMessageConversationComponentID(userID, "participant_user_id"); err != nil {
			return "", err
		}
	}
	if _, err := normalizeMessageOriginInput(&input); err != nil {
		return "", err
	}
	return inputConversationID(input)
}

func normalizeMessageOriginInput(input *CreateMessageInput) (string, error) {
	origin := strings.ToLower(strings.TrimSpace(input.MessageOrigin))
	if origin == "" {
		origin = MessageOriginHuman
	}
	switch origin {
	case MessageOriginHuman, MessageOriginAI, MessageOriginSystem:
	default:
		return "", apperror.InvalidArgument("message_origin must be human, ai, or system")
	}
	input.MessageOrigin = origin
	input.AgentAccountID = strings.TrimSpace(input.AgentAccountID)
	input.TriggerServerMsgID = strings.TrimSpace(input.TriggerServerMsgID)
	input.AgentRunID = strings.TrimSpace(input.AgentRunID)
	if input.AgentAccountID != "" {
		if err := validateMessageConversationComponentID(input.AgentAccountID, "agent_account_id"); err != nil {
			return "", err
		}
	}
	if input.TriggerServerMsgID != "" {
		if err := validateMessageRequiredID(input.TriggerServerMsgID, "trigger_server_msg_id"); err != nil {
			return "", err
		}
	}
	if input.AgentRunID != "" {
		if err := validateMessageRequiredID(input.AgentRunID, "agent_run_id"); err != nil {
			return "", err
		}
	}
	switch origin {
	case MessageOriginAI:
		if input.AgentAccountID == "" {
			input.AgentAccountID = input.SenderID
		}
		if input.AgentAccountID != input.SenderID {
			return "", apperror.InvalidArgument("agent_account_id must match sender_id for ai messages")
		}
	case MessageOriginHuman, MessageOriginSystem:
		if input.AgentAccountID != "" || input.TriggerServerMsgID != "" || input.AgentRunID != "" || input.AllowRecursiveTrigger {
			return "", apperror.InvalidArgument("agent metadata is only allowed for ai messages")
		}
	}
	return origin, nil
}

func normalizeMessagePullRange(fromSeq, toSeq int64, limit int, order string) (int64, int64, int, string, error) {
	if fromSeq < 0 {
		return 0, 0, 0, "", apperror.InvalidArgument("from_seq must be greater than or equal to 0")
	}
	if toSeq < 0 {
		return 0, 0, 0, "", apperror.InvalidArgument("to_seq must be greater than or equal to 0")
	}
	if fromSeq == 0 {
		fromSeq = 1
	}
	if limit < 0 {
		return 0, 0, 0, "", apperror.InvalidArgument("limit must be greater than or equal to 0")
	}
	if limit == 0 {
		limit = defaultMessagePullLimit
	}
	if limit > maxMessagePullLimit {
		limit = maxMessagePullLimit
	}
	order = strings.ToLower(strings.TrimSpace(order))
	if order == "" {
		order = MessageStorageOrderAsc
	}
	if order != MessageStorageOrderAsc && order != MessageStorageOrderDesc {
		return 0, 0, 0, "", apperror.InvalidArgument("order must be asc or desc")
	}
	return fromSeq, toSeq, limit, order, nil
}

func validateMessageRequiredID(value string, field string) error {
	if strings.TrimSpace(value) != value || value == "" {
		return apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > maxMessageIDLength {
		return apperror.InvalidArgument(field + " must be 128 characters or fewer")
	}
	if strings.Contains(value, messageStorageIdempotencyDelim) {
		return apperror.InvalidArgument(field + " cannot contain NUL")
	}
	return nil
}

func validateMessageConversationComponentID(value string, field string) error {
	if err := validateMessageRequiredID(value, field); err != nil {
		return err
	}
	if strings.Contains(value, messageConversationIDDelimiter) {
		return apperror.InvalidArgument(field + " cannot contain ':'")
	}
	return nil
}

func validateMessageConversationID(value string) error {
	if strings.TrimSpace(value) != value || value == "" {
		return apperror.InvalidArgument("conversation_id is required")
	}
	if len([]rune(value)) > maxMessageConversationIDLength {
		return apperror.InvalidArgument("conversation_id must be 256 characters or fewer")
	}
	if strings.Contains(value, messageStorageIdempotencyDelim) {
		return apperror.InvalidArgument("conversation_id cannot contain NUL")
	}
	return nil
}
