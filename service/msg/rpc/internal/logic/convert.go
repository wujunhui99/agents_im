package logic

import (
	"strconv"

	"github.com/wujunhui99/agents_im/service/msg/rpc/internal/model"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
)

// messageToPB 把存库行（goctl Messages）映射成对外 proto Message：int 枚举→string、content 解码、send_time coalesce。
func messageToPB(m *model.Messages) *msg.Message {
	if m == nil {
		return nil
	}
	return &msg.Message{
		ServerMsgId:           strconv.FormatInt(m.MessageId, 10),
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
