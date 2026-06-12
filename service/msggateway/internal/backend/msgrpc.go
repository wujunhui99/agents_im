// Package backend 把 ws.MessageBackend 落到 msg-rpc gRPC（03 §9 A3）。
// 字段映射对位 service/msg/api BFF 的 4 个消息路由；gRPC status 经
// rpcerror.FromStatus 还原为 apperror，保证 ws error envelope 的 code 不变。
package backend

import (
	"context"

	"github.com/wujunhui99/agents_im/common/share/gateway"
	"github.com/wujunhui99/agents_im/common/share/rpcerror"
	msgpb "github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
)

type MsgRPCBackend struct {
	client msgclient.Msg
}

func NewMsgRPCBackend(client msgclient.Msg) *MsgRPCBackend {
	return &MsgRPCBackend{client: client}
}

func (b *MsgRPCBackend) SendMessage(ctx context.Context, req gateway.SendMessageRPCRequest) (gateway.SendMessageRPCResponse, error) {
	result, err := b.client.SendMessage(ctx, &msgpb.SendMessageRequest{
		SenderId:    req.SenderID,
		ReceiverId:  req.ReceiverID,
		GroupId:     req.GroupID,
		ChatType:    req.ChatType,
		ClientMsgId: req.ClientMsgID,
		ContentType: req.ContentType,
		Content:     req.Content,
	})
	if err != nil {
		return gateway.SendMessageRPCResponse{}, rpcerror.FromStatus(err)
	}
	return gateway.SendMessageRPCResponse{
		Message:      pbToSnapshot(result.GetMessage()),
		Deduplicated: result.GetDeduplicated(),
	}, nil
}

func (b *MsgRPCBackend) PullMessages(ctx context.Context, req gateway.PullMessagesRPCRequest) (gateway.PullMessagesRPCResponse, error) {
	result, err := b.client.PullMessages(ctx, &msgpb.PullMessagesRequest{
		UserId:         req.UserID,
		ConversationId: req.ConversationID,
		FromSeq:        req.FromSeq,
		ToSeq:          req.ToSeq,
		Limit:          req.Limit,
		Order:          req.Order,
	})
	if err != nil {
		return gateway.PullMessagesRPCResponse{}, rpcerror.FromStatus(err)
	}
	messages := make([]gateway.MessageSnapshot, 0, len(result.GetMessages()))
	for _, m := range result.GetMessages() {
		messages = append(messages, pbToSnapshot(m))
	}
	return gateway.PullMessagesRPCResponse{
		Messages: messages,
		IsEnd:    result.GetIsEnd(),
		NextSeq:  result.GetNextSeq(),
	}, nil
}

func (b *MsgRPCBackend) GetConversationSeqs(ctx context.Context, req gateway.GetConversationSeqsRPCRequest) (gateway.GetConversationSeqsRPCResponse, error) {
	result, err := b.client.GetConversationsSeqState(ctx, &msgpb.GetConversationsSeqStateRequest{
		UserId:          req.UserID,
		ConversationIds: req.ConversationIDs,
	})
	if err != nil {
		return gateway.GetConversationSeqsRPCResponse{}, rpcerror.FromStatus(err)
	}
	states := make([]gateway.ConversationSeqState, 0, len(result.GetStates()))
	for _, s := range result.GetStates() {
		states = append(states, pbToSeqState(s))
	}
	return gateway.GetConversationSeqsRPCResponse{States: states}, nil
}

func (b *MsgRPCBackend) MarkConversationAsRead(ctx context.Context, req gateway.MarkConversationAsReadRPCRequest) (gateway.MarkConversationAsReadRPCResponse, error) {
	result, err := b.client.MarkConversationAsRead(ctx, &msgpb.MarkConversationAsReadRequest{
		UserId:         req.UserID,
		ConversationId: req.ConversationID,
		HasReadSeq:     req.HasReadSeq,
	})
	if err != nil {
		return gateway.MarkConversationAsReadRPCResponse{}, rpcerror.FromStatus(err)
	}
	return gateway.MarkConversationAsReadRPCResponse{
		ConversationID: result.GetConversationId(),
		HasReadSeq:     result.GetHasReadSeq(),
		MaxSeq:         result.GetMaxSeq(),
		UnreadCount:    result.GetUnreadCount(),
		Updated:        result.GetUpdated(),
	}, nil
}

func pbToSnapshot(m *msgpb.Message) gateway.MessageSnapshot {
	if m == nil {
		return gateway.MessageSnapshot{}
	}
	return gateway.MessageSnapshot{
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

func pbToSeqState(s *msgpb.ConversationSeqState) gateway.ConversationSeqState {
	if s == nil {
		return gateway.ConversationSeqState{}
	}
	state := gateway.ConversationSeqState{
		ConversationID: s.GetConversationId(),
		MaxSeq:         s.GetMaxSeq(),
		HasReadSeq:     s.GetHasReadSeq(),
		UnreadCount:    s.GetUnreadCount(),
		MaxSeqTime:     s.GetMaxSeqTime(),
	}
	if s.GetLastMessage() != nil {
		snapshot := pbToSnapshot(s.GetLastMessage())
		state.LastMessage = &snapshot
	}
	return state
}
