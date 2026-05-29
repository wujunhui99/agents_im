package repository

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	ChatTypeSingle = "single"
	ChatTypeGroup  = "group"

	ContentTypeText  = "text"
	ContentTypeImage = "image"
	ContentTypeFile  = "file"

	MessageOriginHuman  = "human"
	MessageOriginAI     = "ai"
	MessageOriginSystem = "system"
)

const (
	ConversationTypeSingle int16 = 1
	ConversationTypeGroup  int16 = 2

	MessageContentTypeText  int16 = 1
	MessageContentTypeImage int16 = 2
	MessageContentTypeFile  int16 = 3

	MessageOriginHumanValue  int16 = 1
	MessageOriginAIValue     int16 = 2
	MessageOriginSystemValue int16 = 3
)

func conversationTypeValue(chatType string) (int16, error) {
	switch strings.TrimSpace(strings.ToLower(chatType)) {
	case ChatTypeSingle:
		return ConversationTypeSingle, nil
	case ChatTypeGroup:
		return ConversationTypeGroup, nil
	default:
		return 0, apperror.InvalidArgument("chat_type must be single or group")
	}
}

func conversationTypeString(value int16) string {
	switch value {
	case ConversationTypeSingle:
		return ChatTypeSingle
	case ConversationTypeGroup:
		return ChatTypeGroup
	default:
		return strconv.FormatInt(int64(value), 10)
	}
}

func contentTypeValue(contentType string) (int16, error) {
	switch strings.TrimSpace(strings.ToLower(contentType)) {
	case ContentTypeText:
		return MessageContentTypeText, nil
	case ContentTypeImage:
		return MessageContentTypeImage, nil
	case ContentTypeFile:
		return MessageContentTypeFile, nil
	default:
		return 0, apperror.InvalidArgument("content_type must be text, image, or file")
	}
}

func contentTypeString(value int16) string {
	switch value {
	case MessageContentTypeText:
		return ContentTypeText
	case MessageContentTypeImage:
		return ContentTypeImage
	case MessageContentTypeFile:
		return ContentTypeFile
	default:
		return strconv.FormatInt(int64(value), 10)
	}
}

func messageOriginValue(origin string) (int16, error) {
	switch strings.TrimSpace(strings.ToLower(origin)) {
	case "", MessageOriginHuman:
		return MessageOriginHumanValue, nil
	case MessageOriginAI:
		return MessageOriginAIValue, nil
	case MessageOriginSystem:
		return MessageOriginSystemValue, nil
	default:
		return 0, apperror.InvalidArgument("message_origin must be human, ai, or system")
	}
}

func messageOriginString(value int16) string {
	switch value {
	case MessageOriginHumanValue:
		return MessageOriginHuman
	case MessageOriginAIValue:
		return MessageOriginAI
	case MessageOriginSystemValue:
		return MessageOriginSystem
	default:
		return fmt.Sprintf("unknown:%d", value)
	}
}
