package convhosting

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

const (
	chatTypeSingle = "single"

	peerEnabledReason        = "对方已开启 AI 托管，本会话暂时只能由一方开启"
	agentConversationReason  = "与 AI 助手对话时不能开启 AI 托管，请直接向助手发送消息"
	unsupportedGroupChatHint = "AI 托管 V1 暂不支持群聊"
)

// AgentAccountExistenceChecker 判断某账号是否为活跃 Agent（用于禁止托管与 AI 助手的会话）。
type AgentAccountExistenceChecker interface {
	IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error)
}

// AgentAccountExistenceCheckerFunc 把普通函数适配为 AgentAccountExistenceChecker。
type AgentAccountExistenceCheckerFunc func(ctx context.Context, accountID string) (bool, error)

func (f AgentAccountExistenceCheckerFunc) IsActiveAgentAccount(ctx context.Context, accountID string) (bool, error) {
	if f == nil {
		return false, nil
	}
	return f(ctx, accountID)
}

// ConversationAIHostingLogic 持有 AI 托管开关的业务规则（同会话仅一方可开启、不可托管与
// AI 助手的会话等），背靠 Store 数据层。原 internal/logic.ConversationAIHostingLogic 迁入属主。
type ConversationAIHostingLogic struct {
	store         Store
	agentResolver AgentAccountExistenceChecker
}

// NewConversationAIHostingLogic 构建业务规则层。
func NewConversationAIHostingLogic(store Store) *ConversationAIHostingLogic {
	return &ConversationAIHostingLogic{store: store}
}

// WithAgentAccountResolver 注入 Agent 账号解析器，启用“不可托管与 AI 助手会话”规则。
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
	ConversationID    string
	ChatType          string
	Enabled           bool
	Available         bool
	PeerEnabled       bool
	UnavailableReason string
	MaxRecentMessages int
	SummaryEnabled    bool
}

func (l *ConversationAIHostingLogic) GetConversationAIHosting(ctx context.Context, req GetConversationAIHostingRequest) (ConversationAIHostingResponse, error) {
	if l == nil || l.store == nil {
		return ConversationAIHostingResponse{}, apperror.Internal("conversation AI hosting store is not configured")
	}
	ownerID, conversationID, err := validateConversationAIHostingAccess(req.OwnerAccountID, req.ConversationID)
	if err != nil {
		return ConversationAIHostingResponse{}, err
	}
	return l.conversationAIHostingState(ctx, ownerID, conversationID)
}

func (l *ConversationAIHostingLogic) UpdateConversationAIHosting(ctx context.Context, req UpdateConversationAIHostingRequest) (ConversationAIHostingResponse, error) {
	if l == nil || l.store == nil {
		return ConversationAIHostingResponse{}, apperror.Internal("conversation AI hosting store is not configured")
	}
	ownerID, conversationID, err := validateConversationAIHostingAccess(req.OwnerAccountID, req.ConversationID)
	if err != nil {
		return ConversationAIHostingResponse{}, err
	}
	if req.Enabled {
		blocked, err := l.agentConversationAIHostingBlocked(ctx, conversationID)
		if err != nil {
			return ConversationAIHostingResponse{}, err
		}
		if blocked {
			return ConversationAIHostingResponse{}, apperror.InvalidArgument(agentConversationReason)
		}
	}
	if _, err := l.store.SetConversationAIHostingEnabled(ctx, Update{
		OwnerAccountID:    ownerID,
		ConversationID:    conversationID,
		Enabled:           req.Enabled,
		MaxRecentMessages: defaultRecentMessages,
		SummaryEnabled:    false,
	}); err != nil {
		return ConversationAIHostingResponse{}, err
	}
	return l.conversationAIHostingState(ctx, ownerID, conversationID)
}

func (l *ConversationAIHostingLogic) conversationAIHostingState(ctx context.Context, ownerID string, conversationID string) (ConversationAIHostingResponse, error) {
	response := ConversationAIHostingResponse{
		ConversationID:    conversationID,
		ChatType:          chatTypeSingle,
		Available:         true,
		MaxRecentMessages: defaultRecentMessages,
		SummaryEnabled:    false,
	}
	blocked, err := l.agentConversationAIHostingBlocked(ctx, conversationID)
	if err != nil {
		return ConversationAIHostingResponse{}, err
	}
	if blocked {
		response.Available = false
		response.UnavailableReason = agentConversationReason
		return response, nil
	}

	if current, err := l.store.GetConversationAIHostingSetting(ctx, ownerID, conversationID); err == nil {
		response.Enabled = current.Enabled
		response.MaxRecentMessages = current.MaxRecentMessages
		response.SummaryEnabled = current.SummaryEnabled
	} else if apperror.From(err).Code != apperror.CodeNotFound {
		return ConversationAIHostingResponse{}, err
	}

	enabled, err := l.store.GetEnabledConversationAIHosting(ctx, conversationID)
	if err != nil {
		if apperror.From(err).Code == apperror.CodeNotFound {
			return response, nil
		}
		return ConversationAIHostingResponse{}, err
	}
	if enabled.OwnerAccountID != ownerID {
		response.Available = false
		response.PeerEnabled = true
		response.UnavailableReason = peerEnabledReason
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
			return "", "", apperror.InvalidArgument(unsupportedGroupChatHint)
		}
		return "", "", apperror.InvalidArgument("conversation_id must be a direct conversation")
	}
	if ownerAccountID != userA && ownerAccountID != userB {
		return "", "", apperror.Forbidden("caller is not a conversation participant")
	}
	return ownerAccountID, conversationID, nil
}

func (l *ConversationAIHostingLogic) agentConversationAIHostingBlocked(ctx context.Context, conversationID string) (bool, error) {
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
