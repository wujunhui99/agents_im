package orchestrator

import (
	"context"
	"fmt"

	"github.com/wujunhui99/agents_im/pkg/apperror"
)

type MessageSender interface {
	SendMessage(ctx context.Context, req SendMessageRequest) (SendMessageResponse, error)
}

type ResponseWriter interface {
	WriteAgentResponse(ctx context.Context, req AgentResponseRequest) (AgentResponseResult, error)
}

type AgentResponseRequest struct {
	RequestID              string   `json:"request_id"`
	OperationID            string   `json:"operation_id,omitempty"`
	TraceID                string   `json:"trace_id,omitempty"`
	AgentRunID             string   `json:"agent_run_id"`
	AgentUserID            string   `json:"agent_user_id"`
	ConversationID         string   `json:"conversation_id,omitempty"`
	ConversationType       string   `json:"conversation_type"`
	ReceiverUserID         string   `json:"receiver_user_id,omitempty"`
	GroupID                string   `json:"group_id,omitempty"`
	ReplyToMessageID       string   `json:"reply_to_message_id,omitempty"`
	Text                   string   `json:"text"`
	AllowRecursiveTrigger  bool     `json:"allow_recursive_trigger,omitempty"`
	TargetAgentUserIDs     []string `json:"target_agent_user_ids,omitempty"`
	TriggerMessageID       string   `json:"trigger_message_id,omitempty"`
	SourceTriggerRequestID string   `json:"source_trigger_request_id,omitempty"`
}

type AgentResponseResult struct {
	Message      Message        `json:"message"`
	Deduplicated bool                 `json:"deduplicated"`
	Metadata     AgentMessageMetadata `json:"metadata"`
}

type MessageServiceResponseWriter struct {
	sender MessageSender
}

func NewMessageServiceResponseWriter(sender MessageSender) (*MessageServiceResponseWriter, error) {
	if sender == nil {
		return nil, apperror.Internal("message sender is not configured")
	}
	return &MessageServiceResponseWriter{sender: sender}, nil
}

func (w *MessageServiceResponseWriter) WriteAgentResponse(ctx context.Context, req AgentResponseRequest) (AgentResponseResult, error) {
	if w == nil || w.sender == nil {
		return AgentResponseResult{}, apperror.Internal("message sender is not configured")
	}
	sendReq, metadata, expectedConversationID, err := normalizeAgentResponseRequest(req)
	if err != nil {
		return AgentResponseResult{}, err
	}

	resp, err := w.sender.SendMessage(ctx, sendReq)
	if err != nil {
		return AgentResponseResult{}, fmt.Errorf("send agent response through message service: %w", err)
	}
	if err := validateMessageServiceResponse(resp, sendReq, expectedConversationID); err != nil {
		return AgentResponseResult{}, err
	}

	return AgentResponseResult{
		Message:      resp.Message,
		Deduplicated: resp.Deduplicated,
		Metadata:     metadata,
	}, nil
}

func normalizeAgentResponseRequest(req AgentResponseRequest) (SendMessageRequest, AgentMessageMetadata, string, error) {
	requestID, err := normalizeRequired(req.RequestID, "request_id")
	if err != nil {
		return SendMessageRequest{}, AgentMessageMetadata{}, "", err
	}
	agentUserID, err := normalizeRequired(req.AgentUserID, "agent_user_id")
	if err != nil {
		return SendMessageRequest{}, AgentMessageMetadata{}, "", err
	}
	agentRunID, err := normalizeRequired(req.AgentRunID, "agent_run_id")
	if err != nil {
		return SendMessageRequest{}, AgentMessageMetadata{}, "", err
	}
	conversationType, err := normalizeConversationType(req.ConversationType)
	if err != nil {
		return SendMessageRequest{}, AgentMessageMetadata{}, "", err
	}
	text, err := normalizeAgentResponseText(req.Text)
	if err != nil {
		return SendMessageRequest{}, AgentMessageMetadata{}, "", err
	}
	expectedConversationID := normalizeOptional(req.ConversationID)

	sendReq := SendMessageRequest{
		SenderID:              agentUserID,
		ChatType:              conversationType,
		ClientMsgID:           requestID,
		ContentType:           MessageContentTypeText,
		Content:               text,
		MessageOrigin:         MessageOriginAI,
		AgentAccountID:        agentUserID,
		AgentRunID:            agentRunID,
		AllowRecursiveTrigger: req.AllowRecursiveTrigger,
	}
	switch conversationType {
	case ConversationTypeSingle:
		receiverUserID, err := normalizeRequired(req.ReceiverUserID, "receiver_user_id")
		if err != nil {
			return SendMessageRequest{}, AgentMessageMetadata{}, "", err
		}
		if normalizeOptional(req.GroupID) != "" {
			return SendMessageRequest{}, AgentMessageMetadata{}, "", apperror.InvalidArgument("group_id must be empty for single response")
		}
		sendReq.ReceiverID = receiverUserID
	case ConversationTypeGroup:
		groupID, err := normalizeRequired(req.GroupID, "group_id")
		if err != nil {
			return SendMessageRequest{}, AgentMessageMetadata{}, "", err
		}
		if normalizeOptional(req.ReceiverUserID) != "" {
			return SendMessageRequest{}, AgentMessageMetadata{}, "", apperror.InvalidArgument("receiver_user_id must be empty for group response")
		}
		sendReq.GroupID = groupID
	}

	triggerMessageID := normalizeOptional(req.TriggerMessageID)
	if triggerMessageID == "" {
		triggerMessageID = normalizeOptional(req.ReplyToMessageID)
	}
	sendReq.TriggerServerMsgID = triggerMessageID
	return sendReq, AgentMessageMetadata{
		AgentRunID:            agentRunID,
		TriggerMessageID:      triggerMessageID,
		AllowRecursiveTrigger: req.AllowRecursiveTrigger,
	}, expectedConversationID, nil
}

func validateMessageServiceResponse(resp SendMessageResponse, req SendMessageRequest, expectedConversationID string) error {
	message := resp.Message
	if message.ServerMsgID == "" {
		return apperror.Internal("message service returned empty server_msg_id")
	}
	if message.ConversationID == "" {
		return apperror.Internal("message service returned empty conversation_id")
	}
	if expectedConversationID != "" && message.ConversationID != expectedConversationID {
		return apperror.Internal("message service returned mismatched conversation_id")
	}
	// Kafka 写路径（03 §9 B2）的 ACK 不带 seq（异步分配），seq=0 合法；
	// 负数仍是契约违例。agent 已读推进依赖 TriggerSeq（入站消息 seq），不受影响。
	if message.Seq < 0 {
		return apperror.Internal("message service returned invalid seq")
	}
	if message.SenderID != req.SenderID {
		return apperror.Internal("message service returned mismatched sender_id")
	}
	if message.ChatType != req.ChatType {
		return apperror.Internal("message service returned mismatched chat_type")
	}
	if message.MessageOrigin != MessageOriginAI {
		return apperror.Internal("message service returned non-ai origin for agent response")
	}
	if message.AgentAccountID != req.AgentAccountID {
		return apperror.Internal("message service returned mismatched agent_account_id")
	}
	if message.TriggerServerMsgID != req.TriggerServerMsgID {
		return apperror.Internal("message service returned mismatched trigger_server_msg_id")
	}
	if message.AgentRunID != req.AgentRunID {
		return apperror.Internal("message service returned mismatched agent_run_id")
	}
	switch req.ChatType {
	case ConversationTypeSingle:
		if message.ReceiverID != req.ReceiverID {
			return apperror.Internal("message service returned mismatched receiver_id")
		}
	case ConversationTypeGroup:
		if message.GroupID != req.GroupID {
			return apperror.Internal("message service returned mismatched group_id")
		}
	}
	return nil
}
