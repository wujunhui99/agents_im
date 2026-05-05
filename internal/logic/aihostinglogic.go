package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const conversationAIHostingPeerEnabledReason = "对方已开启 AI 托管，本会话暂时只能由一方开启"

type ConversationAIHostingLogic struct {
	repo repository.ConversationAIHostingRepository
}

func NewConversationAIHostingLogic(repo repository.ConversationAIHostingRepository) *ConversationAIHostingLogic {
	return &ConversationAIHostingLogic{repo: repo}
}

type GetConversationAIHostingRequest struct {
	OwnerAccountID string
	ConversationID string
}

type UpdateConversationAIHostingRequest struct {
	OwnerAccountID string
	ConversationID string
	Enabled        bool
}

type ConversationAIHostingResponse struct {
	ConversationID    string `json:"conversationId"`
	ChatType          string `json:"chatType"`
	Enabled           bool   `json:"enabled"`
	Available         bool   `json:"available"`
	PeerEnabled       bool   `json:"peerEnabled"`
	UnavailableReason string `json:"unavailableReason,omitempty"`
	MaxRecentMessages int    `json:"maxRecentMessages"`
	SummaryEnabled    bool   `json:"summaryEnabled"`
}

func (l *ConversationAIHostingLogic) GetConversationAIHosting(ctx context.Context, req GetConversationAIHostingRequest) (ConversationAIHostingResponse, error) {
	if l == nil || l.repo == nil {
		return ConversationAIHostingResponse{}, apperror.Internal("conversation AI hosting repository is not configured")
	}
	ownerID, conversationID, err := validateConversationAIHostingAccess(req.OwnerAccountID, req.ConversationID)
	if err != nil {
		return ConversationAIHostingResponse{}, err
	}
	return l.conversationAIHostingState(ctx, ownerID, conversationID)
}

func (l *ConversationAIHostingLogic) UpdateConversationAIHosting(ctx context.Context, req UpdateConversationAIHostingRequest) (ConversationAIHostingResponse, error) {
	if l == nil || l.repo == nil {
		return ConversationAIHostingResponse{}, apperror.Internal("conversation AI hosting repository is not configured")
	}
	ownerID, conversationID, err := validateConversationAIHostingAccess(req.OwnerAccountID, req.ConversationID)
	if err != nil {
		return ConversationAIHostingResponse{}, err
	}
	if _, err := l.repo.SetConversationAIHostingEnabled(ctx, repository.ConversationAIHostingUpdate{
		OwnerAccountID:    ownerID,
		ConversationID:    conversationID,
		Enabled:           req.Enabled,
		MaxRecentMessages: 30,
		SummaryEnabled:    false,
	}); err != nil {
		return ConversationAIHostingResponse{}, err
	}
	return l.conversationAIHostingState(ctx, ownerID, conversationID)
}

func (l *ConversationAIHostingLogic) conversationAIHostingState(ctx context.Context, ownerID string, conversationID string) (ConversationAIHostingResponse, error) {
	response := ConversationAIHostingResponse{
		ConversationID:    conversationID,
		ChatType:          MessageChatTypeSingle,
		Available:         true,
		MaxRecentMessages: 30,
		SummaryEnabled:    false,
	}

	if current, err := l.repo.GetConversationAIHostingSetting(ctx, ownerID, conversationID); err == nil {
		response.Enabled = current.Enabled
		response.MaxRecentMessages = current.MaxRecentMessages
		response.SummaryEnabled = current.SummaryEnabled
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return ConversationAIHostingResponse{}, err
	}

	enabled, err := l.repo.GetEnabledConversationAIHosting(ctx, conversationID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return response, nil
		}
		return ConversationAIHostingResponse{}, err
	}
	if enabled.OwnerAccountID != ownerID {
		response.Available = false
		response.PeerEnabled = true
		response.UnavailableReason = conversationAIHostingPeerEnabledReason
		response.Enabled = false
	}
	return response, nil
}

func validateConversationAIHostingAccess(ownerAccountID string, conversationID string) (string, string, error) {
	ownerAccountID = strings.TrimSpace(ownerAccountID)
	if ownerAccountID == "" {
		return "", "", apperror.InvalidArgument("owner_account_id is required")
	}
	if strings.Contains(ownerAccountID, ":") || strings.Contains(ownerAccountID, "\x00") {
		return "", "", apperror.InvalidArgument("owner_account_id is invalid")
	}

	conversationID = strings.TrimSpace(conversationID)
	userA, userB, ok := singleConversationParticipants(conversationID)
	if !ok {
		if strings.HasPrefix(conversationID, "group:") {
			return "", "", apperror.InvalidArgument("AI 托管 V1 暂不支持群聊")
		}
		return "", "", apperror.InvalidArgument("conversation_id must be a direct conversation")
	}
	if ownerAccountID != userA && ownerAccountID != userB {
		return "", "", apperror.Forbidden("caller is not a conversation participant")
	}
	return ownerAccountID, conversationID, nil
}
