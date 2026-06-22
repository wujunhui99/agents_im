// Package imadapter is the ONLY place in the agent service that writes back to
// IM (04-agent §3.1). The real implementation is a msg-rpc gRPC client calling
// SendMessage with message_origin=ai — the AI reply then travels the exact same
// Kafka pipeline as a human message and is stopped from re-triggering by the
// consumer-side recursion gate (trigger.Judge step 1, D15 step ①).
//
// It implements orchestrator.MessageSender (the response writer's write-back
// surface), replacing the in-process MessageLogic the retired msg-rpc 回流
// consumer used (D15 step ④): AI replies cross the process boundary by gRPC
// instead of sharing msg-rpc's address space.
package imadapter

import (
	"context"

	"github.com/zeromicro/go-zero/zrpc"

	business "github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msg"
	"github.com/wujunhui99/agents_im/service/msg/rpc/msgclient"
)

// MsgRPCSender writes one agent reply back through msg-rpc SendMessage. AI
// replies and human messages share the same chain; seq is assigned by
// msgtransfer's Redis Malloc, so the ACK carries no seq (the response writer's
// seq>=0 contract tolerates this).
type MsgRPCSender struct {
	client msgclient.Msg
}

// NewMsgRPCSender builds the write-back client from a configured zrpc client.
func NewMsgRPCSender(cli zrpc.Client) *MsgRPCSender {
	return &MsgRPCSender{client: msgclient.NewMsg(cli)}
}

// SendMessage satisfies orchestrator.MessageSender: business request → pb →
// msg-rpc gRPC SendMessage → business response.
func (s *MsgRPCSender) SendMessage(ctx context.Context, req business.SendMessageRequest) (business.SendMessageResponse, error) {
	resp, err := s.client.SendMessage(ctx, &msg.SendMessageRequest{
		SenderId:              req.SenderID,
		ReceiverId:            req.ReceiverID,
		GroupId:               req.GroupID,
		ChatType:              req.ChatType,
		ClientMsgId:           req.ClientMsgID,
		ContentType:           req.ContentType,
		Content:               req.Content,
		MessageOrigin:         req.MessageOrigin,
		AgentAccountId:        req.AgentAccountID,
		TriggerServerMsgId:    req.TriggerServerMsgID,
		AgentRunId:            req.AgentRunID,
		AllowRecursiveTrigger: req.AllowRecursiveTrigger,
	})
	if err != nil {
		return business.SendMessageResponse{}, err
	}
	return business.SendMessageResponse{
		Message:      businessMessageFromPB(resp.GetMessage()),
		Deduplicated: resp.GetDeduplicated(),
	}, nil
}

func businessMessageFromPB(m *msg.Message) business.Message {
	if m == nil {
		return business.Message{}
	}
	return business.Message{
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
