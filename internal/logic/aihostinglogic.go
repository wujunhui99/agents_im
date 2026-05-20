package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const conversationAIHostingPeerEnabledReason = "对方已开启 AI 托管，本会话暂时只能由一方开启"
const conversationAIHostingAgentConversationReason = "与 AI 助手对话时不能开启 AI 托管，请直接向助手发送消息"

type ConversationAIHostingLogic struct {
	repo          repository.ConversationAIHostingRepository
	agentResolver AgentAccountExistenceChecker
}

func NewConversationAIHostingLogic(repo repository.ConversationAIHostingRepository) *ConversationAIHostingLogic {
	return &ConversationAIHostingLogic{repo: repo}
}

type AgentAccountExistenceChecker interface {
	IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error)
}

type AgentAccountExistenceCheckerFunc func(ctx context.Context, accountID string) (bool, error)

func (f AgentAccountExistenceCheckerFunc) IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error) {
	if f == nil {
		return false, nil
	}
	return f(ctx, accountID)
}

func (l *ConversationAIHostingLogic) WithAgentAccountResolver(resolver AgentAccountExistenceChecker) *ConversationAIHostingLogic {
	l.agentResolver = resolver
	return l
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
	if req.Enabled {
		blocked, err := l.agentConversationAIHostingBlocked(ctx, ownerID, conversationID)
		if err != nil {
			return ConversationAIHostingResponse{}, err
		}
		if blocked {
			return ConversationAIHostingResponse{}, apperror.InvalidArgument(conversationAIHostingAgentConversationReason)
		}
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
	blocked, err := l.agentConversationAIHostingBlocked(ctx, ownerID, conversationID)
	if err != nil {
		return ConversationAIHostingResponse{}, err
	}
	if blocked {
		response.Available = false
		response.UnavailableReason = conversationAIHostingAgentConversationReason
		return response, nil
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

func (l *ConversationAIHostingLogic) agentConversationAIHostingBlocked(ctx context.Context, ownerID string, conversationID string) (bool, error) {
	if l == nil || l.agentResolver == nil {
		return false, nil
	}
	userA, userB, ok := singleConversationParticipants(conversationID)
	if !ok {
		return false, nil
	}
	for _, participantID := range []string{userA, userB} {
		isAgent, err := l.agentResolver.IsActiveAgentAccount(ctx, participantID)
		if err != nil {
			return false, err
		}
		if isAgent {
			return true, nil
		}
	}
	return false, nil
}
