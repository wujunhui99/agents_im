package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// messageToPB 把存库行（goctl Messages）映射成对外 proto Message：int 枚举→string、content 解码、send_time coalesce。
func messageToPB(m *model.Messages) *msg.Message {
	if m == nil {
		return nil
	}
	return &msg.Message{
		ServerMsgId:           m.MessageId,
		ClientMsgId:           m.ClientMsgId,
		ConversationId:        m.ConversationId,
		Seq:                   m.Seq,
		SenderId:              m.SenderAccountId,
		ReceiverId:            m.ReceiverAccountId,
		GroupId:               m.GroupId,
		ChatType:              model.ConversationTypeString(m.ConversationType),
		ContentType:           model.ContentTypeString(m.ContentType),
		Content:               model.DecodeMessageContent(m.Content),
		MessageOrigin:         model.MessageOriginString(m.MessageOrigin),
		AgentAccountId:        m.AgentAccountId,
		TriggerServerMsgId:    m.TriggerMessageId,
		AgentRunId:            m.AgentRunId,
		AllowRecursiveTrigger: m.AllowRecursiveTrigger,
		SendTime:              model.MessageSendTime(m),
		CreatedAt:             m.ServerReceivedAt.UTC().UnixMilli(),
	}
}

// messageToBusiness 把存库行映射成 internal 层 Message（AI 托管钩子输入；keystone 例外，
// 待 03-message-pipeline §9 B1 把触发点迁到 msgtransfer 后随钩子一起删除）。字段语义与 messageToPB 一致。
func messageToBusiness(m *model.Messages) business.Message {
	if m == nil {
		return business.Message{}
	}
	return business.Message{
		ServerMsgID:           m.MessageId,
		ClientMsgID:           m.ClientMsgId,
		ConversationID:        m.ConversationId,
		Seq:                   m.Seq,
		SenderID:              m.SenderAccountId,
		ReceiverID:            m.ReceiverAccountId,
		GroupID:               m.GroupId,
		ChatType:              model.ConversationTypeString(m.ConversationType),
		ContentType:           model.ContentTypeString(m.ContentType),
		Content:               model.DecodeMessageContent(m.Content),
		MessageOrigin:         model.MessageOriginString(m.MessageOrigin),
		AgentAccountID:        m.AgentAccountId,
		TriggerServerMsgID:    m.TriggerMessageId,
		AgentRunID:            m.AgentRunId,
		AllowRecursiveTrigger: m.AllowRecursiveTrigger,
		SendTime:              model.MessageSendTime(m),
		CreatedAt:             m.ServerReceivedAt.UTC().UnixMilli(),
	}
}

func aiHostingStateToPB(s business.ConversationAIHostingResponse) *msg.ConversationAIHostingState {
	return &msg.ConversationAIHostingState{
		ConversationId:    s.ConversationID,
		ChatType:          s.ChatType,
		Enabled:           s.Enabled,
		Available:         s.Available,
		PeerEnabled:       s.PeerEnabled,
		UnavailableReason: s.UnavailableReason,
		MaxRecentMessages: int64(s.MaxRecentMessages),
		SummaryEnabled:    s.SummaryEnabled,
	}
}

func seqStateToPB(s model.ConversationSeqState) *msg.ConversationSeqState {
	out := &msg.ConversationSeqState{
		ConversationId: s.ConversationID,
		MaxSeq:         s.MaxSeq,
		HasReadSeq:     s.HasReadSeq,
		UnreadCount:    s.UnreadCount,
		MaxSeqTime:     s.MaxSeqTime,
	}
	if s.LastMessage != nil {
		out.LastMessage = messageToPB(s.LastMessage)
	}
	return out
}
