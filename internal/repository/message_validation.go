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
	return inputConversationID(input)
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
