package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/repository"
)

const adminAIReplayMessageLimit = 500

type AdminAIReplayLogic struct {
	messages repository.AdminMessageRepository
	hook     MessageCreatedHook
}

func NewAdminAIReplayLogic(messages repository.AdminMessageRepository, hook MessageCreatedHook) *AdminAIReplayLogic {
	return &AdminAIReplayLogic{messages: messages, hook: hook}
}

type AdminReplayAgentMessageRequest struct {
	ConversationID string
	ServerMsgID    string
}

type AdminReplayAgentMessageResponse struct {
	ConversationID string       `json:"conversationId"`
	ServerMsgID    string       `json:"serverMsgId"`
	Triggered      bool         `json:"triggered"`
	Skipped        bool         `json:"skipped"`
	Reason         string       `json:"reason,omitempty"`
	Message        AdminMessage `json:"message"`
}

func (l *AdminAIReplayLogic) ReplayAgentMessage(ctx context.Context, req AdminReplayAgentMessageRequest) (AdminReplayAgentMessageResponse, error) {
	if l == nil || l.messages == nil {
		return AdminReplayAgentMessageResponse{}, apperror.Internal("admin AI replay message repository is not configured")
	}
	conversationID, err := normalizeAdminConversationID(req.ConversationID)
	if err != nil {
		return AdminReplayAgentMessageResponse{}, err
	}
	serverMsgID := strings.TrimSpace(req.ServerMsgID)
	if serverMsgID == "" {
		return AdminReplayAgentMessageResponse{}, apperror.InvalidArgument("server_msg_id is required")
	}

	messages, _, _, err := l.messages.GetMessages(ctx, conversationID, 1, 0, adminAIReplayMessageLimit, repository.MessageStorageOrderAsc)
	if err != nil {
		return AdminReplayAgentMessageResponse{}, err
	}
	var trigger repository.Message
	found := false
	for _, message := range messages {
		if message.ServerMsgID == serverMsgID {
			trigger = message
			found = true
		}
		if message.TriggerServerMsgID == serverMsgID && message.MessageOrigin == MessageOriginAI {
			return AdminReplayAgentMessageResponse{
				ConversationID: conversationID,
				ServerMsgID:    serverMsgID,
				Skipped:        true,
				Reason:         "ai response already exists for trigger message",
				Message:        adminMessageFromRepository(message),
			}, nil
		}
	}
	if !found {
		return AdminReplayAgentMessageResponse{}, apperror.NotFound("message not found in conversation")
	}
	if trigger.MessageOrigin != MessageOriginHuman {
		return AdminReplayAgentMessageResponse{}, apperror.InvalidArgument("only human messages can be replayed for AI triggering")
	}
	if trigger.ChatType != MessageChatTypeSingle || strings.TrimSpace(trigger.ReceiverID) == "" {
		return AdminReplayAgentMessageResponse{}, apperror.InvalidArgument("only direct messages to an agent account can be replayed")
	}
	if trigger.ContentType != MessageContentTypeText {
		return AdminReplayAgentMessageResponse{}, apperror.InvalidArgument("only text messages can be replayed for AI triggering")
	}
	if l.hook == nil {
		return AdminReplayAgentMessageResponse{}, apperror.Internal("message created hook is not configured")
	}

	eventID := "admin.replay.message.created:" + trigger.ServerMsgID
	if err := l.hook.OnMessageCreated(ctx, MessageCreatedHookInput{
		EventID: eventID,
		Message: trigger.Clone(),
	}); err != nil {
		return AdminReplayAgentMessageResponse{}, err
	}
	return AdminReplayAgentMessageResponse{
		ConversationID: conversationID,
		ServerMsgID:    serverMsgID,
		Triggered:      true,
		Message:        adminMessageFromRepository(trigger),
	}, nil
}
