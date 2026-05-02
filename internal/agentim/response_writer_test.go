package agentim

import (
	"context"
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/logic"
)

func TestResponseWriterUsesMessageSenderSeam(t *testing.T) {
	sender := &recordingMessageSender{
		resp: logic.SendMessageResponse{
			Message: logic.Message{
				ServerMsgID:        "msg_agent_1",
				ConversationID:     "single:agent_1:user_1",
				Seq:                8,
				SenderID:           "agent_1",
				ReceiverID:         "user_1",
				ChatType:           logic.MessageChatTypeSingle,
				ContentType:        logic.MessageContentTypeText,
				Content:            "answer",
				MessageOrigin:      logic.MessageOriginAI,
				AgentAccountID:     "agent_1",
				TriggerServerMsgID: "msg_user_1",
				AgentRunID:         "run_1",
			},
		},
	}
	writer, err := NewMessageServiceResponseWriter(sender)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	resp, err := writer.WriteAgentResponse(context.Background(), AgentResponseRequest{
		RequestID:        "agent-run-1-response",
		AgentRunID:       "run_1",
		AgentUserID:      "agent_1",
		ConversationType: ConversationTypeSingle,
		ReceiverUserID:   "user_1",
		ReplyToMessageID: "msg_user_1",
		Text:             "answer",
	})
	if err != nil {
		t.Fatalf("write response: %v", err)
	}
	if sender.calls != 1 {
		t.Fatalf("message sender calls = %d, want 1", sender.calls)
	}
	if sender.lastReq.SenderID != "agent_1" || sender.lastReq.ReceiverID != "user_1" {
		t.Fatalf("unexpected message request participants: %+v", sender.lastReq)
	}
	if sender.lastReq.ChatType != logic.MessageChatTypeSingle {
		t.Fatalf("chat type = %q", sender.lastReq.ChatType)
	}
	if sender.lastReq.ClientMsgID != "agent-run-1-response" {
		t.Fatalf("client msg id = %q", sender.lastReq.ClientMsgID)
	}
	if sender.lastReq.ContentType != logic.MessageContentTypeText || sender.lastReq.Content != "answer" {
		t.Fatalf("unexpected content request: %+v", sender.lastReq)
	}
	if sender.lastReq.MessageOrigin != logic.MessageOriginAI ||
		sender.lastReq.AgentAccountID != "agent_1" ||
		sender.lastReq.TriggerServerMsgID != "msg_user_1" ||
		sender.lastReq.AgentRunID != "run_1" {
		t.Fatalf("agent response did not send ai metadata through MessageLogic: %+v", sender.lastReq)
	}
	if resp.Message.ServerMsgID != "msg_agent_1" || resp.Metadata.AgentRunID != "run_1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !resp.Metadata.SuppressesAgentTrigger() {
		t.Fatalf("agent response metadata must suppress recursive triggers by default: %+v", resp.Metadata)
	}
}

func TestResponseWriterRejectsMissingMessageSender(t *testing.T) {
	_, err := NewMessageServiceResponseWriter(nil)
	if err == nil {
		t.Fatal("expected missing sender error")
	}
	if apperror.From(err).Code != apperror.CodeInternal {
		t.Fatalf("error code = %s, want %s", apperror.From(err).Code, apperror.CodeInternal)
	}
}

func TestResponseWriterPropagatesMessageSenderFailure(t *testing.T) {
	senderErr := errors.New("message service unavailable")
	writer, err := NewMessageServiceResponseWriter(&recordingMessageSender{err: senderErr})
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	_, err = writer.WriteAgentResponse(context.Background(), AgentResponseRequest{
		RequestID:        "agent-run-1-response",
		AgentRunID:       "run_1",
		AgentUserID:      "agent_1",
		ConversationType: ConversationTypeSingle,
		ReceiverUserID:   "user_1",
		Text:             "answer",
	})
	if !errors.Is(err, senderErr) {
		t.Fatalf("got error %v, want wrapped sender error", err)
	}
}

func TestResponseWriterRejectsEmptyMessageSenderSuccess(t *testing.T) {
	writer, err := NewMessageServiceResponseWriter(&recordingMessageSender{})
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	_, err = writer.WriteAgentResponse(context.Background(), AgentResponseRequest{
		RequestID:        "agent-run-1-response",
		AgentRunID:       "run_1",
		AgentUserID:      "agent_1",
		ConversationType: ConversationTypeSingle,
		ReceiverUserID:   "user_1",
		Text:             "answer",
	})
	if err == nil {
		t.Fatal("expected empty success rejection")
	}
	if apperror.From(err).Code != apperror.CodeInternal {
		t.Fatalf("error code = %s, want %s", apperror.From(err).Code, apperror.CodeInternal)
	}
}

func TestResponseWriterRejectsInvalidGroupResponseWithoutCallingSender(t *testing.T) {
	sender := &recordingMessageSender{}
	writer, err := NewMessageServiceResponseWriter(sender)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	_, err = writer.WriteAgentResponse(context.Background(), AgentResponseRequest{
		RequestID:        "agent-run-1-response",
		AgentRunID:       "run_1",
		AgentUserID:      "agent_1",
		ConversationType: ConversationTypeGroup,
		Text:             "answer",
	})
	if err == nil {
		t.Fatal("expected missing group_id error")
	}
	if sender.calls != 0 {
		t.Fatalf("message sender calls = %d, want 0", sender.calls)
	}
}

type recordingMessageSender struct {
	calls   int
	lastReq logic.SendMessageRequest
	resp    logic.SendMessageResponse
	err     error
}

func (s *recordingMessageSender) SendMessage(_ context.Context, req logic.SendMessageRequest) (logic.SendMessageResponse, error) {
	s.calls++
	s.lastReq = req
	if s.err != nil {
		return logic.SendMessageResponse{}, s.err
	}
	return s.resp, nil
}
