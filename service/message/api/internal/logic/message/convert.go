package message

import (
	"strings"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/service/message/api/internal/types"
)

func splitCommaQuery(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}

	rawParts := strings.Split(value, ",")
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func toConversationSeqState(state business.ConversationSeqState) types.ConversationSeqState {
	var lastMessage *types.Message
	if state.LastMessage != nil {
		msg := toMessage(*state.LastMessage)
		lastMessage = &msg
	}
	return types.ConversationSeqState{
		ConversationID: state.ConversationID,
		MaxSeq:         state.MaxSeq,
		HasReadSeq:     state.HasReadSeq,
		UnreadCount:    state.UnreadCount,
		MaxSeqTime:     state.MaxSeqTime,
		LastMessage:    lastMessage,
	}
}

func toConversationAIHostingData(state business.ConversationAIHostingResponse) types.ConversationAIHostingData {
	return types.ConversationAIHostingData{
		ConversationID:    state.ConversationID,
		ChatType:          state.ChatType,
		Enabled:           state.Enabled,
		Available:         state.Available,
		PeerEnabled:       state.PeerEnabled,
		UnavailableReason: state.UnavailableReason,
		MaxRecentMessages: int64(state.MaxRecentMessages),
		SummaryEnabled:    state.SummaryEnabled,
	}
}

func toMessage(message business.Message) types.Message {
	return types.Message{
		ServerMsgID:           message.ServerMsgID,
		ClientMsgID:           message.ClientMsgID,
		ConversationID:        message.ConversationID,
		Seq:                   message.Seq,
		SenderID:              message.SenderID,
		ReceiverID:            message.ReceiverID,
		GroupID:               message.GroupID,
		ChatType:              message.ChatType,
		ContentType:           message.ContentType,
		Content:               message.Content,
		MessageOrigin:         message.MessageOrigin,
		AgentAccountID:        message.AgentAccountID,
		TriggerServerMsgID:    message.TriggerServerMsgID,
		AgentRunID:            message.AgentRunID,
		AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		SendTime:              message.SendTime,
		CreatedAt:             message.CreatedAt,
	}
}
