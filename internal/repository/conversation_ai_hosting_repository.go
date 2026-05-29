package repository

import (
	"context"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	ConversationAIHostingModeAutoReply         = "auto_reply"
	defaultConversationAIHostingRecentMessages = 30
	maxConversationAIHostingRecentMessages     = 30
)

type ConversationAIHostingSetting struct {
	OwnerAccountID    string    `json:"owner_account_id"`
	ConversationID    string    `json:"conversation_id"`
	Enabled           bool      `json:"enabled"`
	Mode              string    `json:"mode"`
	MaxRecentMessages int       `json:"max_recent_messages"`
	SummaryEnabled    bool      `json:"summary_enabled"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (s ConversationAIHostingSetting) Clone() ConversationAIHostingSetting {
	s.CreatedAt = utcOrZeroRepositoryTime(s.CreatedAt)
	s.UpdatedAt = utcOrZeroRepositoryTime(s.UpdatedAt)
	return s
}

type ConversationAIHostingUpdate struct {
	OwnerAccountID    string
	ConversationID    string
	Enabled           bool
	MaxRecentMessages int
	SummaryEnabled    bool
}

type ConversationAIHostingRepository interface {
	GetConversationAIHostingSetting(ctx context.Context, ownerAccountID string, conversationID string) (ConversationAIHostingSetting, error)
	GetEnabledConversationAIHosting(ctx context.Context, conversationID string) (ConversationAIHostingSetting, error)
	SetConversationAIHostingEnabled(ctx context.Context, input ConversationAIHostingUpdate) (ConversationAIHostingSetting, error)
}

func normalizeConversationAIHostingUpdate(input ConversationAIHostingUpdate) (ConversationAIHostingUpdate, error) {
	ownerAccountID, conversationID, err := normalizeConversationAIHostingOwnerAndConversation(input.OwnerAccountID, input.ConversationID)
	if err != nil {
		return ConversationAIHostingUpdate{}, err
	}
	input.OwnerAccountID = ownerAccountID
	input.ConversationID = conversationID
	input.MaxRecentMessages = normalizeConversationAIHostingRecentLimit(input.MaxRecentMessages)
	return input, nil
}

func normalizeConversationAIHostingOwnerAndConversation(ownerAccountID string, conversationID string) (string, string, error) {
	ownerAccountID, err := normalizeAgentHostingComponentID(ownerAccountID, "owner_account_id")
	if err != nil {
		return "", "", err
	}
	conversationID, err = normalizeAgentHostingRequired(conversationID, "conversation_id")
	if err != nil {
		return "", "", err
	}
	return ownerAccountID, conversationID, nil
}

func normalizeConversationAIHostingSetting(input ConversationAIHostingSetting) (ConversationAIHostingSetting, error) {
	ownerAccountID, conversationID, err := normalizeConversationAIHostingOwnerAndConversation(input.OwnerAccountID, input.ConversationID)
	if err != nil {
		return ConversationAIHostingSetting{}, err
	}
	input.OwnerAccountID = ownerAccountID
	input.ConversationID = conversationID
	input.Mode = normalizeConversationAIHostingMode(input.Mode)
	input.MaxRecentMessages = normalizeConversationAIHostingRecentLimit(input.MaxRecentMessages)
	return input, nil
}

func normalizeConversationAIHostingMode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ConversationAIHostingModeAutoReply
	}
	return value
}

func normalizeConversationAIHostingRecentLimit(value int) int {
	if value <= 0 {
		return defaultConversationAIHostingRecentMessages
	}
	if value > maxConversationAIHostingRecentMessages {
		return maxConversationAIHostingRecentMessages
	}
	return value
}

func conversationAIHostingConflictError() error {
	return apperror.AlreadyExists("对方已开启 AI 托管，本会话暂时只能由一方开启")
}
