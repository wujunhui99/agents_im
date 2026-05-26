package logic

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/repository"
)

type recordingMessageCreatedHook struct {
	calls int
	last  MessageCreatedHookInput
}

func (h *recordingMessageCreatedHook) OnMessageCreated(_ context.Context, input MessageCreatedHookInput) error {
	h.calls++
	h.last = input
	return nil
}

func TestAdminAIReplayLogicReplaysDirectHumanMessageToAgent(t *testing.T) {
	ctx := context.Background()
	messages := repository.NewMemoryMessageRepository()
	hook := &recordingMessageCreatedHook{}
	logic := NewAdminAIReplayLogic(messages, hook)

	trigger, _, err := messages.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:    "usr_new_registration",
		ReceiverID:  "agent_default_assistant_account",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-new-user-ai-assistant",
		ContentType: MessageContentTypeText,
		Content:     "hello",
	})
	if err != nil {
		t.Fatalf("create trigger message: %v", err)
	}

	resp, err := logic.ReplayAgentMessage(ctx, AdminReplayAgentMessageRequest{
		ConversationID: trigger.ConversationID,
		ServerMsgID:    trigger.ServerMsgID,
	})
	if err != nil {
		t.Fatalf("replay agent message: %v", err)
	}
	if !resp.Triggered || resp.Skipped {
		t.Fatalf("replay response = %+v, want triggered and not skipped", resp)
	}
	if hook.calls != 1 {
		t.Fatalf("hook calls = %d, want 1", hook.calls)
	}
	if hook.last.EventID != "admin.replay.message.created:"+trigger.ServerMsgID {
		t.Fatalf("event id = %q", hook.last.EventID)
	}
	if hook.last.Message.ServerMsgID != trigger.ServerMsgID || hook.last.Message.ReceiverID != "agent_default_assistant_account" {
		t.Fatalf("hook message = %+v, want replayed trigger", hook.last.Message)
	}
}

func TestAdminAIReplayLogicSkipsWhenAIResponseAlreadyExists(t *testing.T) {
	ctx := context.Background()
	messages := repository.NewMemoryMessageRepository()
	hook := &recordingMessageCreatedHook{}
	logic := NewAdminAIReplayLogic(messages, hook)

	trigger, _, err := messages.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:    "usr_new_registration",
		ReceiverID:  "agent_default_assistant_account",
		ChatType:    MessageChatTypeSingle,
		ClientMsgID: "client-new-user-ai-assistant",
		ContentType: MessageContentTypeText,
		Content:     "hello",
	})
	if err != nil {
		t.Fatalf("create trigger message: %v", err)
	}
	if _, _, err := messages.CreateMessageIdempotent(ctx, repository.CreateMessageInput{
		SenderID:           "agent_default_assistant_account",
		ReceiverID:         "usr_new_registration",
		ChatType:           MessageChatTypeSingle,
		ClientMsgID:        "ai-response",
		ContentType:        MessageContentTypeText,
		Content:            "hi",
		MessageOrigin:      MessageOriginAI,
		TriggerServerMsgID: trigger.ServerMsgID,
	}); err != nil {
		t.Fatalf("create AI response: %v", err)
	}

	resp, err := logic.ReplayAgentMessage(ctx, AdminReplayAgentMessageRequest{
		ConversationID: trigger.ConversationID,
		ServerMsgID:    trigger.ServerMsgID,
	})
	if err != nil {
		t.Fatalf("replay agent message: %v", err)
	}
	if !resp.Skipped || resp.Triggered || resp.Reason == "" {
		t.Fatalf("replay response = %+v, want skipped duplicate", resp)
	}
	if hook.calls != 0 {
		t.Fatalf("hook calls = %d, want 0", hook.calls)
	}
}
