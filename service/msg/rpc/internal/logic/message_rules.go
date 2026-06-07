package logic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
)

// mediaValidator 是 image/file 消息的跨域媒体校验入口（keystone 例外，由 svcCtx.Media 注入）。
type mediaValidator interface {
	ValidateMessageMedia(ctx context.Context, ownerUserID, contentType, content string) error
}

// 消息域输入校验 / 规范化，移植自 internal/logic/messagelogic.go + message_validation.go。
// Phase 0 行为对齐旧实现（保留 normalize：客户端可能未规范化）。

// normalizedSend 是 SendMessage 校验/解析后的内部输入。
type normalizedSend struct {
	SenderID              string
	ReceiverID            string
	GroupID               string
	ChatType              string
	ClientMsgID           string
	ContentType           string
	Content               string
	MessageOrigin         string
	AgentAccountID        string
	TriggerServerMsgID    string
	AgentRunID            string
	AllowRecursiveTrigger bool
	ParticipantUserIDs    []string
	ConversationID        string
}

func normalizeMessageRequiredID(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 128 {
		return "", apperror.InvalidArgument(field + " must be 128 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument(field + " cannot contain NUL")
	}
	return value, nil
}

func normalizeMessageConversationComponentID(value, field string) (string, error) {
	value, err := normalizeMessageRequiredID(value, field)
	if err != nil {
		return "", err
	}
	if strings.Contains(value, ":") {
		return "", apperror.InvalidArgument(field + " cannot contain ':'")
	}
	return value, nil
}

func normalizeConversationID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument("conversation_id is required")
	}
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument("conversation_id must be 256 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument("conversation_id cannot contain NUL")
	}
	return value, nil
}

func normalizePullRange(fromSeq, toSeq int64, limit int, order string) (int64, int64, int, string, error) {
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
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	order = strings.ToLower(strings.TrimSpace(order))
	if order == "" {
		order = model.OrderAsc
	}
	if order != model.OrderAsc && order != model.OrderDesc {
		return 0, 0, 0, "", apperror.InvalidArgument("order must be asc or desc")
	}
	return fromSeq, toSeq, limit, order, nil
}

func singleConversationParticipants(conversationID string) (string, string, bool) {
	const prefix = "single:"
	if !strings.HasPrefix(conversationID, prefix) {
		return "", "", false
	}
	parts := strings.Split(conversationID, ":")
	if len(parts) != 3 {
		return "", "", false
	}
	userA := strings.TrimSpace(parts[1])
	userB := strings.TrimSpace(parts[2])
	if userA == "" || userB == "" {
		return "", "", false
	}
	return userA, userB, true
}

func groupIDFromConversationID(conversationID string) (string, bool) {
	const prefix = "group:"
	if !strings.HasPrefix(conversationID, prefix) {
		return "", false
	}
	groupID := strings.TrimSpace(strings.TrimPrefix(conversationID, prefix))
	return groupID, groupID != ""
}

// normalizeMessageContent 校验/规范化 content（image/file 需 media 校验），移植自 messagelogic.go。
func normalizeMessageContent(ctx context.Context, media mediaValidator, senderID, rawContentType, rawContent string) (string, string, error) {
	contentType := strings.ToLower(strings.TrimSpace(rawContentType))
	switch contentType {
	case model.ContentTypeText:
		content := strings.TrimSpace(rawContent)
		if content == "" {
			return "", "", apperror.InvalidArgument("content is required")
		}
		if len([]rune(content)) > 4096 {
			return "", "", apperror.InvalidArgument("content must be 4096 characters or fewer")
		}
		return contentType, content, nil
	case model.ContentTypeImage, model.ContentTypeFile:
		content := strings.TrimSpace(rawContent)
		if content == "" {
			return "", "", apperror.InvalidArgument("content is required")
		}
		if len([]rune(content)) > 8192 {
			return "", "", apperror.InvalidArgument("content must be 8192 characters or fewer")
		}
		if !json.Valid([]byte(content)) {
			return "", "", apperror.InvalidArgument("content must be valid JSON for image/file messages")
		}
		if media == nil {
			return "", "", apperror.Internal("media validator is not configured")
		}
		if err := media.ValidateMessageMedia(ctx, senderID, contentType, content); err != nil {
			return "", "", err
		}
		return contentType, content, nil
	default:
		return "", "", apperror.InvalidArgument("content_type must be text, image, or file")
	}
}

// applyMessageOriginMetadata 校验/规范化 message_origin + agent 元数据，移植自 messagelogic.go。
func applyMessageOriginMetadata(input *normalizedSend, origin, agentAccountID, triggerServerMsgID, agentRunID string, allowRecursiveTrigger bool) error {
	origin = strings.ToLower(strings.TrimSpace(origin))
	if origin == "" {
		origin = model.MessageOriginHuman
	}
	switch origin {
	case model.MessageOriginHuman, model.MessageOriginAI, model.MessageOriginSystem:
	default:
		return apperror.InvalidArgument("message_origin must be human, ai, or system")
	}
	input.MessageOrigin = origin
	input.AgentAccountID = strings.TrimSpace(agentAccountID)
	input.TriggerServerMsgID = strings.TrimSpace(triggerServerMsgID)
	input.AgentRunID = strings.TrimSpace(agentRunID)
	input.AllowRecursiveTrigger = allowRecursiveTrigger
	if origin != model.MessageOriginAI {
		if input.AgentAccountID != "" || input.TriggerServerMsgID != "" || input.AgentRunID != "" || input.AllowRecursiveTrigger {
			return apperror.InvalidArgument("agent metadata is only allowed for ai messages")
		}
		return nil
	}
	if input.AgentAccountID == "" {
		input.AgentAccountID = input.SenderID
	}
	if input.AgentAccountID != input.SenderID {
		return apperror.InvalidArgument("agent_account_id must match sender_id for ai messages")
	}
	return nil
}

// messagePayloadHash 复刻 internal/repository.messagePayloadHash（幂等冲突判定基准）。
func messagePayloadHash(in normalizedSend) string {
	payload := struct {
		SenderID              string `json:"sender_id"`
		ClientMsgID           string `json:"client_msg_id"`
		ConversationID        string `json:"conversation_id"`
		ChatType              string `json:"chat_type"`
		ReceiverID            string `json:"receiver_id"`
		GroupID               string `json:"group_id"`
		ContentType           string `json:"content_type"`
		Content               string `json:"content"`
		MessageOrigin         string `json:"message_origin"`
		AgentAccountID        string `json:"agent_account_id,omitempty"`
		TriggerServerMsgID    string `json:"trigger_server_msg_id,omitempty"`
		AgentRunID            string `json:"agent_run_id,omitempty"`
		AllowRecursiveTrigger bool   `json:"allow_recursive_trigger,omitempty"`
	}{
		SenderID:              in.SenderID,
		ClientMsgID:           in.ClientMsgID,
		ConversationID:        in.ConversationID,
		ChatType:              in.ChatType,
		ReceiverID:            in.ReceiverID,
		GroupID:               in.GroupID,
		ContentType:           in.ContentType,
		Content:               in.Content,
		MessageOrigin:         in.MessageOrigin,
		AgentAccountID:        in.AgentAccountID,
		TriggerServerMsgID:    in.TriggerServerMsgID,
		AgentRunID:            in.AgentRunID,
		AllowRecursiveTrigger: in.AllowRecursiveTrigger,
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
