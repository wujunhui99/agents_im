package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogictest"

	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/registrytest"
	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
)

func TestConversationAIHostingRuntimeRequestBuilderUsesBoundedRecentMessages(t *testing.T) {
	ctx := context.Background()
	im := newFakeIM()
	conversationID := singleConvID("usr_a", "usr_b")
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
		im.appendHuman(Message{
			ConversationID: conversationID,
			ClientMsgID:    "seed-context-" + string(rune('0'+seq)),
			SenderID:       sender,
			ReceiverID:     receiver,
			ChatType:       MessageChatTypeSingle,
			ContentType:    MessageContentTypeText,
			Content:        content,
		})
	}

	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageHistory:    im,
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
		SourceContentType: MessageContentTypeText,
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

// TestBuilderHydratesPromptTextFromTriggerMessage 锁定实时 Kafka 触发链路回归：
// consumer.agentTriggerFromJudged 只携带路由元数据、不带 PromptText，构建器须从消息历史
// 回填触发消息正文。回填前 NormalizeRunRequest 会以 "prompt_text is required" 失败，对用户
// 表现为兜底文案“服务暂时不可用”（trace 排查实证）。
func TestBuilderHydratesPromptTextFromTriggerMessage(t *testing.T) {
	ctx := context.Background()
	im := newFakeIM()
	conversationID := singleConvID("agent_acc", "usr_peer")
	sent := im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "kafka-trigger-msg",
		SenderID:       "usr_peer",
		ReceiverID:     "agent_acc",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "111",
	})

	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageHistory: im,
		DeepSeek:       config.DeepSeekConfig{Model: "deepseek-test"},
	})
	// 镜像 agentTriggerFromJudged 的输出：无 PromptText / 无 SourceMessageText。
	trigger := AgentTrigger{
		RequestID:          "evt-1:agent_acc",
		EventID:            "evt-1",
		TriggerType:        TriggerTypeUserPrivateMessage,
		AgentUserID:        "agent_acc",
		RequestingUserID:   "usr_peer",
		ConversationID:     conversationID,
		ConversationType:   ConversationTypeSingle,
		TriggerMessageID:   sent.ServerMsgID,
		TriggerSeq:         sent.Seq,
		ReplyToMessageID:   sent.ServerMsgID,
		SourceMessageID:    sent.ServerMsgID,
		SourceMessageSeq:   sent.Seq,
		SourceContentType:  MessageContentTypeText,
		TargetAgentUserIDs: []string{"agent_acc"},
	}
	req, err := builder.BuildRuntimeRequest(ctx, trigger)
	if err != nil {
		t.Fatalf("build runtime request: %v", err)
	}
	if req.PromptText != "111" {
		t.Fatalf("prompt_text = %q, want hydrated trigger message %q", req.PromptText, "111")
	}
	// 回填后整条链路（含 NormalizeRunRequest + trigger 一致性校验）必须通过——这正是修复前
	// 失败为 runtime_request_invalid: prompt_text is required 的地方。
	if _, err := normalizeRuntimeRequestForTrigger(req, trigger); err != nil {
		t.Fatalf("normalize runtime request for trigger: %v", err)
	}
}

func TestConversationAIHostingRuntimeRequestBuilderRejectsNonSingleConversation(t *testing.T) {
	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageHistory: newFakeIM(),
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
	im := newFakeIM()
	agentRepo := agentlogictest.NewMemoryAgentStore()
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
	conversationID := singleConvID("agent_creator", "usr_new")
	im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "agent-creator-profile-msg",
		SenderID:       "usr_new",
		ReceiverID:     "agent_creator",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "你能做什么？",
	})

	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageHistory:    im,
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
		SourceContentType:  MessageContentTypeText,
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
	im := newFakeIM()
	agentRepo := agentlogictest.NewMemoryAgentStore()
	registry := registrytest.NewMemoryStore()
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
	conversationID := singleConvID("agent_runtime_account", "usr_peer")
	im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "runtime-definition-msg",
		SenderID:       "usr_peer",
		ReceiverID:     "agent_runtime_account",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "hello runtime agent",
	})
	builder := NewConversationAIHostingRuntimeRequestBuilder(ConversationAIHostingRuntimeRequestBuilderConfig{
		MessageHistory:  im,
		AgentRepository: agentRepo,
		AgentRegistry:   registry,
		DeepSeek:        config.DeepSeekConfig{Model: "deepseek-test"},
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
		SourceContentType:  MessageContentTypeText,
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
