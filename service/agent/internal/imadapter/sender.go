// Package imadapter is the ONLY place in the agent service that writes back
// to IM (04-agent §3.1). The real implementation is a msg-rpc gRPC client
// calling SendMessage with message_origin=ai — the AI reply then travels the
// exact same Kafka pipeline as a human message and is stopped from
// re-triggering by the consumer-side recursion gate (D15 step ①).
package imadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"

	"github.com/wujunhui99/agents_im/pkg/messaging"
)

// SendAgentMessageRequest mirrors the msg-rpc SendMessage surface for an
// AI-origin message.
type SendAgentMessageRequest struct {
	AgentAccountID string
	ReceiverID     string // single chat: the human peer
	GroupID        string // group chat
	ChatType       string
	ClientMsgID    string
	ContentType    string
	Content        json.RawMessage
	// TriggerServerMsgID / AgentRunID thread the audit chain through the reply.
	TriggerServerMsgID    string
	AgentRunID            string
	AllowRecursiveTrigger bool
}

type SendResult struct {
	ServerMsgID  string
	Deduplicated bool
}

// MessageSender writes one agent reply back to IM.
type MessageSender interface {
	SendAgentMessage(ctx context.Context, req SendAgentMessageRequest) (SendResult, error)
}

// Mock is the scaffold MessageSender (issue #503): validates and logs the
// would-be write-back, sends NOTHING. This keeps service/agent free of side
// effects while the transitional msg-rpc 回流 consumer still produces the real
// AI replies (D15 step ④ swaps ownership).
type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) SendAgentMessage(ctx context.Context, req SendAgentMessageRequest) (SendResult, error) {
	if req.AgentAccountID == "" {
		return SendResult{}, fmt.Errorf("mock sender requires agent_account_id")
	}
	switch req.ChatType {
	case messaging.ChatTypeSingle:
		if req.ReceiverID == "" {
			return SendResult{}, fmt.Errorf("mock sender requires receiver_id for single chat")
		}
	case messaging.ChatTypeGroup:
		if req.GroupID == "" {
			return SendResult{}, fmt.Errorf("mock sender requires group_id for group chat")
		}
	default:
		return SendResult{}, fmt.Errorf("mock sender: unsupported chat_type %q", req.ChatType)
	}
	logx.WithContext(ctx).Infof(
		"agent imadapter [mock, NOT sent]: agent=%s chat_type=%s receiver=%s group=%s trigger=%s run=%s",
		req.AgentAccountID, req.ChatType, req.ReceiverID, req.GroupID, req.TriggerServerMsgID, req.AgentRunID)
	return SendResult{ServerMsgID: "mocksrv-" + req.ClientMsgID}, nil
}
