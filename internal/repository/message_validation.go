package repository

import (
	"encoding/json"
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
	switch input.ContentType {
	case ContentTypeText:
		if strings.TrimSpace(input.Content) == "" {
			return "", apperror.InvalidArgument("content is required")
		}
		if len([]rune(input.Content)) > maxMessageContentLength {
			return "", apperror.InvalidArgument("content must be 4096 characters or fewer")
		}
	case ContentTypeImage:
		if err := validateImageMessageContent(input.Content); err != nil {
			return "", err
		}
	case ContentTypeFile:
		if err := validateFileMessageContent(input.Content); err != nil {
			return "", err
		}
	default:
		return "", apperror.InvalidArgument("content_type must be text, image, or file")
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

func validateImageMessageContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return apperror.InvalidArgument("content is required")
	}
	var body struct {
		MediaID string `json:"mediaId"`
	}
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return apperror.InvalidArgument("image content must be a JSON object")
	}
	if strings.TrimSpace(body.MediaID) == "" {
		return apperror.InvalidArgument("image content mediaId is required")
	}
	return nil
}

func validateFileMessageContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return apperror.InvalidArgument("content is required")
	}
	var body struct {
		MediaID     string `json:"mediaId"`
		Filename    string `json:"filename"`
		SizeBytes   int64  `json:"sizeBytes"`
		ContentType string `json:"contentType"`
	}
	if err := json.Unmarshal([]byte(content), &body); err != nil {
		return apperror.InvalidArgument("file content must be a JSON object")
	}
	if strings.TrimSpace(body.MediaID) == "" {
		return apperror.InvalidArgument("file content mediaId is required")
	}
	if strings.TrimSpace(body.Filename) == "" {
		return apperror.InvalidArgument("file content filename is required")
	}
	if body.SizeBytes <= 0 {
		return apperror.InvalidArgument("file content sizeBytes must be positive")
	}
	if strings.TrimSpace(body.ContentType) == "" {
		return apperror.InvalidArgument("file content contentType is required")
	}
	return nil
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
