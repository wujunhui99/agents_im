package msg

import (
	"strings"

	agentpb "github.com/wujunhui99/agents_im/service/agent/rpc/agent"
	"github.com/wujunhui99/agents_im/service/msg/api/internal/types"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// splitCommaQuery 把逗号分隔的 query 参数拆成去空去重前的 id 列表（移植自旧 message-api handler）。
func splitCommaQuery(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func pbToMessage(m *msgpb.Message) types.Message {
	if m == nil {
		return types.Message{}
	}
	return types.Message{
		ServerMsgID:           m.GetServerMsgId(),
		ClientMsgID:           m.GetClientMsgId(),
		ConversationID:        m.GetConversationId(),
		Seq:                   m.GetSeq(),
		SenderID:              m.GetSenderId(),
		ReceiverID:            m.GetReceiverId(),
		GroupID:               m.GetGroupId(),
		ChatType:              m.GetChatType(),
		ContentType:           m.GetContentType(),
		Content:               m.GetContent(),
		MessageOrigin:         m.GetMessageOrigin(),
		AgentAccountID:        m.GetAgentAccountId(),
		TriggerServerMsgID:    m.GetTriggerServerMsgId(),
		AgentRunID:            m.GetAgentRunId(),
		AllowRecursiveTrigger: m.GetAllowRecursiveTrigger(),
		SendTime:              m.GetSendTime(),
		CreatedAt:             m.GetCreatedAt(),
	}
}

func pbToSeqState(s *msgpb.ConversationSeqState) types.ConversationSeqState {
	state := types.ConversationSeqState{
		ConversationID: s.GetConversationId(),
		MaxSeq:         s.GetMaxSeq(),
		HasReadSeq:     s.GetHasReadSeq(),
		UnreadCount:    s.GetUnreadCount(),
		MaxSeqTime:     s.GetMaxSeqTime(),
	}
	if s.GetLastMessage() != nil {
		last := pbToMessage(s.GetLastMessage())
		state.LastMessage = &last
	}
	return state
}

func pbToAIHostingData(s *agentpb.ConversationAIHostingState) types.ConversationAIHostingData {
	if s == nil {
		return types.ConversationAIHostingData{}
	}
	return types.ConversationAIHostingData{
		ConversationID:    s.GetConversationId(),
		ChatType:          s.GetChatType(),
		Enabled:           s.GetEnabled(),
		Available:         s.GetAvailable(),
		PeerEnabled:       s.GetPeerEnabled(),
		UnavailableReason: s.GetUnavailableReason(),
		MaxRecentMessages: s.GetMaxRecentMessages(),
		SummaryEnabled:    s.GetSummaryEnabled(),
	}
}
