package agentim

import (
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
)

const (
	ConversationTypeSingle = "single"
	ConversationTypeGroup  = "group"

	SenderTypeUser  = "user"
	SenderTypeAgent = "agent"

	ContentTypeText = "text"

	TriggerTypeUserPrivateMessage = "user_private_message"
	TriggerTypeGroupMention       = "group_mention"
	TriggerTypeAdminManualRun     = "admin_manual_run"
)

type MessageCreatedEvent struct {
	EventID            string          `json:"event_id"`
	OperationID        string          `json:"operation_id,omitempty"`
	TraceID            string          `json:"trace_id,omitempty"`
	ConversationID     string          `json:"conversation_id"`
	ConversationType   string          `json:"conversation_type"`
	Message            MessageEnvelope `json:"message"`
	TargetAgentUserIDs []string        `json:"target_agent_user_ids,omitempty"`
}

type MessageEnvelope struct {
	ServerMsgID   string               `json:"server_msg_id"`
	ClientMsgID   string               `json:"client_msg_id,omitempty"`
	Seq           int64                `json:"seq"`
	SenderID      string               `json:"sender_id"`
	SenderType    string               `json:"sender_type"`
	ReceiverID    string               `json:"receiver_id,omitempty"`
	GroupID       string               `json:"group_id,omitempty"`
	ContentType   string               `json:"content_type"`
	Text          string               `json:"text"`
	AtUserIDs     []string             `json:"at_user_ids,omitempty"`
	AgentMetadata AgentMessageMetadata `json:"agent_metadata,omitempty"`
}

type AgentMessageMetadata struct {
	AgentRunID            string `json:"agent_run_id,omitempty"`
	TriggerMessageID      string `json:"trigger_message_id,omitempty"`
	AllowRecursiveTrigger bool   `json:"allow_recursive_trigger,omitempty"`
}

func (m AgentMessageMetadata) SuppressesAgentTrigger() bool {
	return !m.AllowRecursiveTrigger
}

type TriggerPolicy struct {
	AllowAgentMessageRecursion bool `json:"allow_agent_message_recursion,omitempty"`
}

type AgentTrigger struct {
	RequestID          string   `json:"request_id"`
	EventID            string   `json:"event_id,omitempty"`
	OperationID        string   `json:"operation_id,omitempty"`
	TraceID            string   `json:"trace_id,omitempty"`
	TriggerType        string   `json:"trigger_type"`
	AgentUserID        string   `json:"agent_user_id"`
	RequestingUserID   string   `json:"requesting_user_id"`
	ConversationID     string   `json:"conversation_id"`
	ConversationType   string   `json:"conversation_type"`
	TriggerMessageID   string   `json:"trigger_message_id,omitempty"`
	TriggerSeq         int64    `json:"trigger_seq,omitempty"`
	PromptText         string   `json:"prompt_text,omitempty"`
	ReplyToMessageID   string   `json:"reply_to_message_id,omitempty"`
	RecursiveTrigger   bool     `json:"recursive_trigger,omitempty"`
	SourceAgentRunID   string   `json:"source_agent_run_id,omitempty"`
	SourceAgentUserID  string   `json:"source_agent_user_id,omitempty"`
	SourceMessageID    string   `json:"source_message_id,omitempty"`
	SourceMessageSeq   int64    `json:"source_message_seq,omitempty"`
	SourceMessageText  string   `json:"source_message_text,omitempty"`
	SourceContentType  string   `json:"source_content_type,omitempty"`
	TargetAgentUserIDs []string `json:"target_agent_user_ids,omitempty"`
}

type AdminManualRunRequest struct {
	RequestID        string `json:"request_id"`
	OperationID      string `json:"operation_id,omitempty"`
	TraceID          string `json:"trace_id,omitempty"`
	AdminUserID      string `json:"admin_user_id"`
	AgentUserID      string `json:"agent_user_id"`
	ConversationID   string `json:"conversation_id"`
	ConversationType string `json:"conversation_type"`
	PromptText       string `json:"prompt_text"`
}

func normalizeRequired(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument(field + " must be 256 characters or fewer")
	}
	return value, nil
}

func normalizeOptional(value string) string {
	return strings.TrimSpace(value)
}

func normalizeConversationType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case ConversationTypeSingle, ConversationTypeGroup:
		return value, nil
	default:
		return "", apperror.InvalidArgument("conversation_type must be single or group")
	}
}

func normalizeSenderType(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case SenderTypeUser, SenderTypeAgent:
		return value, nil
	default:
		return "", apperror.InvalidArgument("sender_type must be user or agent")
	}
}

func containsID(values []string, target string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func uniqueNonEmptyIDs(values []string) []string {
	ids := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}
