package logic

import (
	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/proto/messagepb"
)

func toMessage(message business.Message) *messagepb.Message {
	return &messagepb.Message{
		ServerMsgId:           message.ServerMsgID,
		ClientMsgId:           message.ClientMsgID,
		ConversationId:        message.ConversationID,
		Seq:                   message.Seq,
		SenderId:              message.SenderID,
		ReceiverId:            message.ReceiverID,
		GroupId:               message.GroupID,
		ChatType:              message.ChatType,
		ContentType:           message.ContentType,
		Content:               message.Content,
		MessageOrigin:         message.MessageOrigin,
		AgentAccountId:        message.AgentAccountID,
		TriggerServerMsgId:    message.TriggerServerMsgID,
		AgentRunId:            message.AgentRunID,
		AllowRecursiveTrigger: message.AllowRecursiveTrigger,
		SendTime:              message.SendTime,
		CreatedAt:             message.CreatedAt,
	}
}

func toConversationSeqState(state business.ConversationSeqState) *messagepb.ConversationSeqState {
	result := &messagepb.ConversationSeqState{
		ConversationId: state.ConversationID,
		MaxSeq:         state.MaxSeq,
		HasReadSeq:     state.HasReadSeq,
		UnreadCount:    state.UnreadCount,
		MaxSeqTime:     state.MaxSeqTime,
	}
	if state.LastMessage != nil {
		result.LastMessage = toMessage(*state.LastMessage)
	}
	return result
}
