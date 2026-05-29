package repository

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	AgentTriggerStatusRunning   = "running"
	AgentTriggerStatusSucceeded = "succeeded"
	AgentTriggerStatusFailed    = "failed"

	DefaultAgentTriggerRunningTTL = 10 * time.Minute
)

type AgentConversationHosting struct {
	ConversationID             string    `json:"conversation_id"`
	AgentAccountID             string    `json:"agent_account_id"`
	Enabled                    bool      `json:"enabled"`
	AllowAgentMessageRecursion bool      `json:"allow_agent_message_recursion"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

func (h AgentConversationHosting) Clone() AgentConversationHosting {
	h.CreatedAt = utcOrZeroRepositoryTime(h.CreatedAt)
	h.UpdatedAt = utcOrZeroRepositoryTime(h.UpdatedAt)
	return h
}

type AgentTriggerStartInput struct {
	IdempotencyKey     string
	ConversationID     string
	AgentAccountID     string
	TriggerServerMsgID string
	TriggerEventID     string
	RunningTTL         time.Duration
}

type AgentTriggerFinishInput struct {
	IdempotencyKey      string
	Status              string
	ResponseServerMsgID string
	ErrorMessage        string
}

type AgentConversationHostingRepository interface {
	UpsertAgentConversationHosting(ctx context.Context, hosting AgentConversationHosting) (AgentConversationHosting, error)
	GetAgentConversationHosting(ctx context.Context, conversationID string) (AgentConversationHosting, error)
	TryStartAgentTrigger(ctx context.Context, input AgentTriggerStartInput) (bool, error)
	FinishAgentTrigger(ctx context.Context, input AgentTriggerFinishInput) error
}

func normalizeAgentConversationHosting(input AgentConversationHosting) (AgentConversationHosting, error) {
	conversationID, err := normalizeAgentHostingRequired(input.ConversationID, "conversation_id")
	if err != nil {
		return AgentConversationHosting{}, err
	}
	agentAccountID, err := normalizeAgentHostingComponentID(input.AgentAccountID, "agent_account_id")
	if err != nil {
		return AgentConversationHosting{}, err
	}
	input.ConversationID = conversationID
	input.AgentAccountID = agentAccountID
	return input, nil
}

func normalizeAgentTriggerStartInput(input AgentTriggerStartInput) (AgentTriggerStartInput, error) {
	var err error
	input.IdempotencyKey, err = normalizeAgentHostingRequired(input.IdempotencyKey, "idempotency_key")
	if err != nil {
		return AgentTriggerStartInput{}, err
	}
	input.ConversationID, err = normalizeAgentHostingRequired(input.ConversationID, "conversation_id")
	if err != nil {
		return AgentTriggerStartInput{}, err
	}
	input.AgentAccountID, err = normalizeAgentHostingComponentID(input.AgentAccountID, "agent_account_id")
	if err != nil {
		return AgentTriggerStartInput{}, err
	}
	input.TriggerServerMsgID, err = normalizeAgentHostingRequired(input.TriggerServerMsgID, "trigger_server_msg_id")
	if err != nil {
		return AgentTriggerStartInput{}, err
	}
	input.TriggerEventID = strings.TrimSpace(input.TriggerEventID)
	input.RunningTTL = normalizeAgentTriggerRunningTTL(input.RunningTTL)
	return input, nil
}

func normalizeAgentTriggerRunningTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultAgentTriggerRunningTTL
	}
	if ttl < time.Millisecond {
		return time.Millisecond
	}
	return ttl
}

func normalizeAgentTriggerFinishInput(input AgentTriggerFinishInput) (AgentTriggerFinishInput, error) {
	var err error
	input.IdempotencyKey, err = normalizeAgentHostingRequired(input.IdempotencyKey, "idempotency_key")
	if err != nil {
		return AgentTriggerFinishInput{}, err
	}
	input.Status = strings.ToLower(strings.TrimSpace(input.Status))
	switch input.Status {
	case AgentTriggerStatusSucceeded:
		input.ResponseServerMsgID, err = normalizeAgentHostingRequired(input.ResponseServerMsgID, "response_server_msg_id")
		if err != nil {
			return AgentTriggerFinishInput{}, err
		}
		input.ErrorMessage = ""
	case AgentTriggerStatusFailed:
		input.ResponseServerMsgID = ""
		input.ErrorMessage = strings.TrimSpace(input.ErrorMessage)
	default:
		return AgentTriggerFinishInput{}, apperror.InvalidArgument("agent trigger status must be succeeded or failed")
	}
	return input, nil
}

func normalizeAgentHostingRequired(value string, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", apperror.InvalidArgument(field + " is required")
	}
	if len([]rune(value)) > 256 {
		return "", apperror.InvalidArgument(field + " must be 256 characters or fewer")
	}
	if strings.Contains(value, "\x00") {
		return "", apperror.InvalidArgument(field + " cannot contain NUL")
	}
	return value, nil
}

func normalizeAgentHostingComponentID(value string, field string) (string, error) {
	value, err := normalizeAgentHostingRequired(value, field)
	if err != nil {
		return "", err
	}
	if strings.Contains(value, ":") {
		return "", apperror.InvalidArgument(field + " cannot contain ':'")
	}
	return value, nil
}

func utcOrZeroRepositoryTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}
	return value.UTC()
}
