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
		RequestID:         "req-1",
		EventID:           "evt-1",
		TriggerType:       TriggerTypeUserPrivateMessage,
		AgentUserID:       "usr_a",
		RequestingUserID:  "usr_b",
		ConversationID:    conversationID,
		ConversationType:  ConversationTypeSingle,
		TriggerMessageID:  "msg_5",
		TriggerSeq:        5,
		PromptText:        clearTask,
		ReplyToMessageID:  "msg_5",
		SourceMessageID:   "msg_5",
		SourceMessageSeq:  5,
		SourceMessageText: clearTask,
		SourceContentType: logic.MessageContentTypeText,
	})
	if err != nil {
		t.Fatalf("build runtime request: %v", err)
	}
	if got := len(req.Conversation); got != 3 {
		t.Fatalf("conversation len = %d, want bounded recent 3", got)
	}
	if req.Conversation[0].Seq != 3 || req.Conversation[2].Seq != 5 {
		t.Fatalf("conversation seqs = %+v, want 3..5", req.Conversation)
	}
	if !strings.Contains(req.Agent.Prompt.Content, "当前用户") {
		t.Fatalf("default system prompt = %q, want AI hosting prompt", req.Agent.Prompt.Content)
	}
	if req.Agent.Model.Provider != "deepseek" || req.Agent.Model.Model != "deepseek-test" {
		t.Fatalf("model config = %+v", req.Agent.Model)
	}
}

func TestConversationAIHostingRuntimeRequestBuilderRejectsNonSingleConversation(t *testing.T) {
	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageRepository: repository.NewMemoryMessageRepository(),
	})
	_, err := builder.BuildRuntimeRequest(context.Background(), AgentTrigger{
		ConversationType: ConversationTypeGroup,
	})
	if err == nil || !strings.Contains(err.Error(), "direct conversations") {
		t.Fatalf("BuildRuntimeRequest error = %v, want direct conversation rejection", err)
	}
}

func TestBuilderUsesStoredAgentProfileWhenTriggerTargetsAgentAccount(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	agentRepo := repository.NewMemoryAgentRepository()
	agent, err := agentRepo.CreateAgent(ctx, model.Agent{
		AgentID:     "agent_default_assistant",
		AccountID:   "agent_creator",
		Name:        "agent_creator",
		Description: "Default general AI assistant",
		Status:      agentruntime.AgentStatusActive,
		CreatedBy:   "system",
	})
	if err != nil {
		t.Fatal(err)
	}
	conversationID := repository.SingleConversationID("agent_creator", "usr_new")
	if _, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_new",
		ReceiverID:  "agent_creator",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "agent-creator-profile-msg",
		ContentType: logic.MessageContentTypeText,
		Content:     "你能做什么？",
	}); err != nil {
		t.Fatal(err)
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
	if req.Agent.AgentID != agent.AgentID || req.Agent.Name != "agent_creator" || req.Agent.Description != "Default general AI assistant" {
		t.Fatalf("runtime request did not use stored agent profile: %+v", req.Agent)
	}
}

func TestBuilderUsesStoredAgentRuntimeDefinition(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	registry := repository.NewMemoryAgentRegistryRepository()
	agent, err := agentRepo.CreateAgent(ctx, model.Agent{
		AgentID:     "agent_runtime_definition",
		AccountID:   "agent_runtime_account",
		Name:        "Runtime Agent",
		Description: "Configured runtime definition",
		Status:      model.AgentStatusActive,
		CreatedBy:   "usr_owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := registry.CreatePrompt(ctx, model.AgentPrompt{
		PromptID:            "prompt_runtime_definition",
		Name:                "runtime_prompt",
		Content:             "Use the configured runtime prompt.",
		VariablesSchemaJSON: "{}",
		Version:             "v1",
		Status:              model.AgentPromptStatusActive,
		CreatedBy:           "usr_owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.BindPrompt(ctx, model.AgentPromptBinding{AgentID: agent.AgentID, PromptID: prompt.PromptID, CreatedBy: "usr_owner"}); err != nil {
		t.Fatal(err)
	}
	tool, err := registry.RegisterTool(ctx, model.AgentTool{
		ToolID:           "tool_runtime_context",
		Name:             model.LocalToolHandlerGetConversationContext,
		Description:      "Read recent context",
		ToolType:         model.AgentToolTypeLocal,
		LocalHandlerKey:  model.LocalToolHandlerGetConversationContext,
		InputSchemaJSON:  `{"type":"object"}`,
		OutputSchemaJSON: `{"type":"object"}`,
		PermissionLevel:  "read_only",
		Status:           model.AgentToolStatusActive,
		AdminConfigured:  true,
		CreatedBy:        "usr_owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := registry.BindTool(ctx, model.AgentToolBinding{AgentID: agent.AgentID, ToolID: tool.ToolID, CreatedBy: "usr_owner"}); err != nil {
		t.Fatal(err)
	}
	messageLogic := logic.NewMessageLogic(messageRepo)
	conversationID := repository.SingleConversationID("agent_runtime_account", "usr_peer")
	if _, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_peer",
		ReceiverID:  "agent_runtime_account",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "runtime-definition-msg",
		ContentType: logic.MessageContentTypeText,
		Content:     "hello runtime agent",
	}); err != nil {
		t.Fatal(err)
	}
	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageRepository: messageRepo,
		AgentRepository:   agentRepo,
		AgentRegistry:     registry,
		DeepSeek:          config.DeepSeekConfig{Model: "deepseek-test"},
	})
	req, err := builder.BuildRuntimeRequest(ctx, AgentTrigger{
		RequestID:          "message.created:runtime-definition-msg:agent_runtime_account",
		EventID:            "message.created:runtime-definition-msg",
		TriggerType:        TriggerTypeUserPrivateMessage,
		AgentUserID:        "agent_runtime_account",
		RequestingUserID:   "usr_peer",
		ConversationID:     conversationID,
		ConversationType:   ConversationTypeSingle,
		TriggerMessageID:   "msg_runtime_definition",
		TriggerSeq:         1,
		SourceMessageSeq:   1,
		SourceContentType:  logic.MessageContentTypeText,
		TargetAgentUserIDs: []string{"agent_runtime_account"},
	})
	if err != nil {
		t.Fatalf("build runtime request: %v", err)
	}
	if req.Agent.Prompt.PromptID != prompt.PromptID || req.Agent.Prompt.Content != prompt.Content {
		t.Fatalf("runtime prompt = %+v, want configured prompt", req.Agent.Prompt)
	}
	if len(req.Agent.Tools) != 1 || req.Agent.Tools[0].ToolID != tool.ToolID {
		t.Fatalf("runtime tools = %+v, want configured tool", req.Agent.Tools)
	}
}
