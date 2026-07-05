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
	case model.ContentTypeAt:
		content := strings.TrimSpace(rawContent)
		if content == "" {
			return "", "", apperror.InvalidArgument("content is required")
		}
		if len([]rune(content)) > 8192 {
			return "", "", apperror.InvalidArgument("content must be 8192 characters or fewer")
		}
		if _, err := parseAtContent(content); err != nil {
			return "", "", err
		}
		return contentType, content, nil
	default:
		return "", "", apperror.InvalidArgument("content_type must be text, image, file, or at")
	}
}

const maxAtUserCount = 50

// atMessageContent 是 `at` 消息 content 的入站结构（OpenIM AtText 语义）：真正决定
// @ 的是 atUserList，text 仅用于展示；atUsersInfo/isAtSelf 由前端携带、后端透传不校验。
type atMessageContent struct {
	Text       string   `json:"text"`
	AtUserList []string `json:"atUserList"`
}

// parseAtContent 校验并解析 `at` content：text 非空(≤4096 runes)、atUserList 为非空
// 字符串数组(去空去重、≤maxAtUserCount、每个 ≤128 字符)。返回去重后的被 @ id 列表。
func parseAtContent(content string) ([]string, error) {
	if !json.Valid([]byte(content)) {
		return nil, apperror.InvalidArgument("content must be valid JSON for at messages")
	}
	var parsed atMessageContent
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, apperror.InvalidArgument("content must be valid JSON for at messages")
	}
	text := strings.TrimSpace(parsed.Text)
	if text == "" {
		return nil, apperror.InvalidArgument("at message text is required")
	}
	if len([]rune(text)) > 4096 {
		return nil, apperror.InvalidArgument("at message text must be 4096 characters or fewer")
	}
	ids := make([]string, 0, len(parsed.AtUserList))
	seen := make(map[string]struct{}, len(parsed.AtUserList))
	for _, raw := range parsed.AtUserList {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if len([]rune(id)) > 128 {
			return nil, apperror.InvalidArgument("atUserList entry must be 128 characters or fewer")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, apperror.InvalidArgument("atUserList must contain at least one user id")
	}
	if len(ids) > maxAtUserCount {
		return nil, apperror.InvalidArgument("atUserList must contain 50 user ids or fewer")
	}
	return ids, nil
}

// extractAtUserIDs 从已校验的 `at` content 里取出被 @ 的 id 列表（供 Kafka 事件 payload
// AtUserIDs 使用）；非 `at` 或解析失败返回 nil。
func extractAtUserIDs(contentType, content string) []string {
	if contentType != model.ContentTypeAt {
		return nil
	}
	ids, err := parseAtContent(content)
	if err != nil {
		return nil
	}
	return ids
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
