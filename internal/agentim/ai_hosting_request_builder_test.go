package agentim

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestConversationAIHostingRuntimeRequestBuilderUsesBoundedRecentMessages(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	conversationID := repository.SingleConversationID("usr_a", "usr_b")
	clearTask := "你能帮我对比一下 Python 和 Go 语言的性能吗？"
	for seq := 1; seq <= 5; seq++ {
		sender, receiver := "usr_a", "usr_b"
		if seq%2 == 1 {
			sender, receiver = "usr_b", "usr_a"
		}
		content := "message " + string(rune('0'+seq))
		if seq == 5 {
			content = clearTask
		}
		if _, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
			SenderID:    sender,
			ReceiverID:  receiver,
			ChatType:    logic.MessageChatTypeSingle,
			ClientMsgID: "seed-context-" + string(rune('0'+seq)),
			ContentType: logic.MessageContentTypeText,
			Content:     content,
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
		PromptText:         clearTask,
		ReplyToMessageID:   "msg_000005",
		SourceMessageID:    "msg_000005",
		SourceMessageSeq:   5,
		SourceMessageText:  clearTask,
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
	if req.PromptText != clearTask || req.SourceMessageText != clearTask {
		t.Fatalf("trigger text missing from runtime request: prompt=%q source=%q", req.PromptText, req.SourceMessageText)
	}
	last := req.Conversation[len(req.Conversation)-1]
	if last.ServerMsgID != "msg_000005" || last.Seq != 5 || last.Text != clearTask {
		t.Fatalf("latest trigger message is not the final bounded context message: %+v", req.Conversation)
	}
	if req.Metadata["summary_used"] != "false" || req.Metadata["recent_message_count"] != "3" {
		t.Fatalf("summary/context metadata mismatch: %+v", req.Metadata)
	}
	prompt := req.Agent.Prompt.Content
	for _, want := range []string{"明确", "直接", "不要只回复"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("AI hosting default prompt does not guard clear tasks from vague follow-up; missing %q in %q", want, prompt)
		}
	}
	for _, message := range req.Conversation {
		if message.ContentType != agentruntime.ContentTypeText {
			t.Fatalf("unexpected context content type: %+v", message)
		}
	}
}

func TestConversationAIHostingRuntimeRequestBuilderUsesAgentProfileWhenAvailable(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	agentRepo := repository.NewMemoryAgentRepository()
	if _, err := agentRepo.CreateAgent(ctx, model.Agent{
		AgentID:     "agent_default_assistant",
		AccountID:   "agent_creator",
		IMUserID:    "agent_creator",
		Name:        "agent_creator",
		Description: "Default general AI assistant",
		Status:      model.AgentStatusActive,
	}); err != nil {
		t.Fatalf("create agent profile: %v", err)
	}
	conversationID := repository.SingleConversationID("usr_new", "agent_creator")
	if _, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_new",
		ReceiverID:  "agent_creator",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "ask-agent-profile",
		ContentType: logic.MessageContentTypeText,
		Content:     "你能做什么？",
	}); err != nil {
		t.Fatalf("seed private agent message: %v", err)
	}

	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageRepository: messageRepo,
		AgentRepository:   agentRepo,
		DeepSeek:          config.DeepSeekConfig{Model: "deepseek-test"},
		MaxRecentMessages: 3,
	})
	req, err := builder.BuildRuntimeRequest(ctx, AgentTrigger{
		RequestID:          "message.created:msg_000001:agent_creator",
		EventID:            "message.created:msg_000001",
		TriggerType:        TriggerTypeUserPrivateMessage,
		AgentUserID:        "agent_creator",
		RequestingUserID:   "usr_new",
		ConversationID:     conversationID,
		ConversationType:   ConversationTypeSingle,
		TriggerMessageID:   "msg_000001",
		TriggerSeq:         1,
		PromptText:         "你能做什么？",
		ReplyToMessageID:   "msg_000001",
		SourceMessageID:    "msg_000001",
		SourceMessageSeq:   1,
		SourceMessageText:  "你能做什么？",
		SourceContentType:  logic.MessageContentTypeText,
		TargetAgentUserIDs: []string{"agent_creator"},
	})
	if err != nil {
		t.Fatalf("build runtime request: %v", err)
	}
	if req.Agent.AgentID != "agent_default_assistant" || req.Agent.Name != "agent_creator" || req.Agent.Description != "Default general AI assistant" {
		t.Fatalf("runtime request did not use stored agent profile: %+v", req.Agent)
	}
}
