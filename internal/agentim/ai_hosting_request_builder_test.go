package agentim

import (
	"context"
	"testing"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestConversationAIHostingRuntimeRequestBuilderUsesBoundedRecentMessages(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	conversationID := repository.SingleConversationID("usr_a", "usr_b")
	for seq := 1; seq <= 5; seq++ {
		sender, receiver := "usr_a", "usr_b"
		if seq%2 == 1 {
			sender, receiver = "usr_b", "usr_a"
		}
		if _, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
			SenderID:    sender,
			ReceiverID:  receiver,
			ChatType:    logic.MessageChatTypeSingle,
			ClientMsgID: "seed-context-" + string(rune('0'+seq)),
			ContentType: logic.MessageContentTypeText,
			Content:     "message " + string(rune('0'+seq)),
		}); err != nil {
			t.Fatalf("seed message %d: %v", seq, err)
		}
	}

	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageRepository: messageRepo,
		DeepSeek:          config.DeepSeekConfig{Model: "deepseek-test"},
		MaxRecentMessages: 3,
	})
	req, err := builder.BuildRuntimeRequest(ctx, AgentTrigger{
		RequestID:          "message.created:msg_000005:usr_a",
		EventID:            "message.created:msg_000005",
		TriggerType:        TriggerTypeUserPrivateMessage,
		AgentUserID:        "usr_a",
		RequestingUserID:   "usr_b",
		ConversationID:     conversationID,
		ConversationType:   ConversationTypeSingle,
		TriggerMessageID:   "msg_000005",
		TriggerSeq:         5,
		PromptText:         "message 5",
		ReplyToMessageID:   "msg_000005",
		SourceMessageID:    "msg_000005",
		SourceMessageSeq:   5,
		SourceMessageText:  "message 5",
		SourceContentType:  logic.MessageContentTypeText,
		TargetAgentUserIDs: []string{"usr_a"},
	})
	if err != nil {
		t.Fatalf("build runtime request: %v", err)
	}
	if req.AgentUserID != "usr_a" || req.Agent.AgentUserID != "usr_a" {
		t.Fatalf("owner not preserved in runtime request: %+v", req)
	}
	if req.Agent.Model.Provider != "deepseek" || req.Agent.Model.Model != "deepseek-test" {
		t.Fatalf("model config mismatch: %+v", req.Agent.Model)
	}
	if len(req.Conversation) != 3 {
		t.Fatalf("recent context count = %d, want 3: %+v", len(req.Conversation), req.Conversation)
	}
	if req.Conversation[0].Seq != 3 || req.Conversation[2].Seq != 5 {
		t.Fatalf("context window is not the last 3 messages: %+v", req.Conversation)
	}
	if req.Metadata["summary_used"] != "false" || req.Metadata["recent_message_count"] != "3" {
		t.Fatalf("summary/context metadata mismatch: %+v", req.Metadata)
	}
	for _, message := range req.Conversation {
		if message.ContentType != agentruntime.ContentTypeText {
			t.Fatalf("unexpected context content type: %+v", message)
		}
	}
}
