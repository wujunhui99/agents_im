package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agentlogictest"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/model"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/agaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/aghosting"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/config"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/convhosting"
	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
)

func TestPrivateAgentChatTriggersAgentReply(t *testing.T) {
	ctx := context.Background()
	im := newFakeIM()
	hostingRepo := aghosting.NewMemoryStore()
	aiHostingStore := convhosting.NewMemoryStore()
	agentRepo := agentlogictest.NewMemoryAgentStore()
	auditStore := agaudit.NewMemoryStore()
	writer, err := NewMessageServiceResponseWriter(im)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	if _, err := agentRepo.CreateAgent(ctx, model.Agent{
		AgentID:   "agent_default_assistant",
		AccountID: "agent_creator",
		IMUserID:  "agent_creator",
		Name:      "agent_creator",
		Status:    model.AgentStatusActive,
	}); err != nil {
		t.Fatalf("create default assistant agent: %v", err)
	}

	runtimeCalls := 0
	runtime := runtimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		if req.AgentUserID != "agent_creator" || req.RequestingUserID != "usr_new" {
			t.Fatalf("runtime request used wrong agent/user: %+v", req)
		}
		return agentruntime.RunResult{
			RunID:     req.RunID,
			FinalText: "我是 AI 助手，有什么可以帮你？",
		}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: runtimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  agaudit.NewRunRecorder(auditStore),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:           hostingRepo,
		AIHostingStore:       aiHostingStore,
		Runner:               orchestrator,
		AgentAccountResolver: NewAgentRepositoryAccountResolver(agentRepo),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}

	conversationID := singleConvID("usr_new", "agent_creator")
	human := im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "ask-agent-creator",
		SenderID:       "usr_new",
		ReceiverID:     "agent_creator",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "你是谁？",
		MessageOrigin:  MessageOriginHuman,
	})
	if _, err := hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID: "evt_private_agent_1",
		Message: human,
	}); err != nil {
		t.Fatalf("handle private agent message: %v", err)
	}

	messages := waitForConversationCount(t, im, conversationID, 2)
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	reply := messages[1]
	if reply.MessageOrigin != MessageOriginAI || reply.SenderID != "agent_creator" || reply.AgentAccountID != "agent_creator" {
		t.Fatalf("reply did not use agent_creator ai metadata: %+v", reply)
	}
	if reply.ReceiverID != "usr_new" || reply.TriggerServerMsgID != human.ServerMsgID {
		t.Fatalf("reply routing/trigger metadata mismatch: trigger=%+v reply=%+v", human, reply)
	}
}

func TestConversationAIHostingSlowGenerationDoesNotBlockSendAndMarksReadFirst(t *testing.T) {
	ctx := context.Background()
	im := newFakeIM()
	hostingRepo := aghosting.NewMemoryStore()
	aiHostingStore := convhosting.NewMemoryStore()
	auditStore := agaudit.NewMemoryStore()
	writer, err := NewMessageServiceResponseWriter(im)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	runtimeCalls := 0
	releaseRuntime := make(chan struct{})
	runtime := runtimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		if req.AgentUserID != "usr_a" || req.RequestingUserID != "usr_b" {
			t.Errorf("runtime request used wrong hosting owner/requester: %+v", req)
		}
		<-releaseRuntime
		return agentruntime.RunResult{
			RunID:     req.RunID,
			FinalText: "托管回复",
		}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: runtimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  agaudit.NewRunRecorder(auditStore),
		Writer: writer,
		Now: func() time.Time {
			return time.Unix(200, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:     hostingRepo,
		AIHostingStore: aiHostingStore,
		Runner:         orchestrator,
		ReadMarker:     NewConversationReadMarker(im),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}

	conversationID := singleConvID("usr_a", "usr_b")
	if _, err := aiHostingStore.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    "usr_a",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable AI hosting: %v", err)
	}

	human := im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "human-peer-trigger",
		SenderID:       "usr_b",
		ReceiverID:     "usr_a",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "你好",
		MessageOrigin:  MessageOriginHuman,
	})

	// HandleMessageCreated 同步完成幂等占位 + 已读推进后即返回，AI 生成在后台异步进行（不阻塞）。
	if _, err := hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID: "evt_slow_gen_1",
		Message: human,
	}); err != nil {
		close(releaseRuntime)
		t.Fatalf("handle hosted trigger: %v", err)
	}

	// 已读在 AI 生成前就应推进到触发消息 seq。
	if got := im.readSeq("usr_a", conversationID); got != human.Seq {
		close(releaseRuntime)
		t.Fatalf("hosted owner read seq before AI completion = %d, want %d", got, human.Seq)
	}
	// AI 生成尚被阻塞，会话里只应有人类触发消息。
	if got := im.messages(conversationID); len(got) != 1 {
		close(releaseRuntime)
		t.Fatalf("messages before AI completion = %+v, want only human trigger", got)
	}

	close(releaseRuntime)
	messages := waitForConversationCount(t, im, conversationID, 2)
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	reply := messages[1]
	if reply.MessageOrigin != MessageOriginAI || reply.SenderID != "usr_a" || reply.AgentAccountID != "usr_a" {
		t.Fatalf("reply did not use hosted owner ai metadata: %+v", reply)
	}
	if reply.TriggerServerMsgID != human.ServerMsgID {
		t.Fatalf("reply trigger metadata = %q, want %q", reply.TriggerServerMsgID, human.ServerMsgID)
	}
}

func TestConversationAIHostingDuplicateTriggerDoesNotQueueDuplicateReply(t *testing.T) {
	ctx := context.Background()
	im := newFakeIM()
	hostingRepo := aghosting.NewMemoryStore()
	aiHostingStore := convhosting.NewMemoryStore()
	auditStore := agaudit.NewMemoryStore()
	writer, err := NewMessageServiceResponseWriter(im)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	runtimeCalls := 0
	releaseRuntime := make(chan struct{})
	runtime := runtimeFunc(func(context.Context, agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		<-releaseRuntime
		return agentruntime.RunResult{RunID: "run_hosted_1", FinalText: "托管回复"}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: runtimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  agaudit.NewRunRecorder(auditStore),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:     hostingRepo,
		AIHostingStore: aiHostingStore,
		Runner:         orchestrator,
		ReadMarker:     NewConversationReadMarker(im),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}

	conversationID := singleConvID("usr_a", "usr_b")
	if _, err := aiHostingStore.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    "usr_a",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable AI hosting: %v", err)
	}

	human := im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "human-peer-trigger",
		SenderID:       "usr_b",
		ReceiverID:     "usr_a",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "你好",
		MessageOrigin:  MessageOriginHuman,
	})
	// 同一事件（相同 EventID → 相同 trigger 幂等键）重复投递，只应调度一次 run。
	triggerInput := ConversationHostingMessageCreatedInput{EventID: "evt_dup_1", Message: human}
	if _, err := hosting.HandleMessageCreated(ctx, triggerInput); err != nil {
		close(releaseRuntime)
		t.Fatalf("handle first trigger: %v", err)
	}
	dup, err := hosting.HandleMessageCreated(ctx, triggerInput)
	if err != nil {
		close(releaseRuntime)
		t.Fatalf("handle duplicate trigger: %v", err)
	}
	if dup.Triggered {
		close(releaseRuntime)
		t.Fatalf("duplicate trigger scheduled another run: %+v", dup)
	}

	close(releaseRuntime)
	messages := waitForConversationCount(t, im, conversationID, 2)
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	aiReplies := 0
	for _, message := range messages {
		if message.MessageOrigin == MessageOriginAI {
			aiReplies++
		}
	}
	if aiReplies != 1 {
		t.Fatalf("ai replies = %d, messages=%+v", aiReplies, messages)
	}
}

func TestConversationAIHostingMissingProviderDoesNotBlockOriginalSendAndNotifiesUser(t *testing.T) {
	ctx := context.Background()
	im := newFakeIM()
	hostingRepo := aghosting.NewMemoryStore()
	aiHostingStore := convhosting.NewMemoryStore()
	auditStore := agaudit.NewMemoryStore()
	writer, err := NewMessageServiceResponseWriter(im)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}
	missingProviderErr := config.ErrDeepSeekAPIKeyMissing
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtimeFunc(func(context.Context, agentruntime.RunRequest) (agentruntime.RunResult, error) {
			return agentruntime.RunResult{}, missingProviderErr
		}),
		RequestBuilder: runtimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  agaudit.NewRunRecorder(auditStore),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:     hostingRepo,
		AIHostingStore: aiHostingStore,
		Runner:         orchestrator,
		ReadMarker:     NewConversationReadMarker(im),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}

	conversationID := singleConvID("usr_a", "usr_b")
	if _, err := aiHostingStore.SetConversationAIHostingEnabled(ctx, convhosting.Update{
		OwnerAccountID:    "usr_a",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable AI hosting: %v", err)
	}

	im.appendHuman(Message{
		ConversationID: conversationID,
		ClientMsgID:    "human-trigger-missing-provider",
		SenderID:       "usr_b",
		ReceiverID:     "usr_a",
		ChatType:       MessageChatTypeSingle,
		ContentType:    MessageContentTypeText,
		Content:        "需要托管回复",
		MessageOrigin:  MessageOriginHuman,
	})
	if _, err := hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID: "evt_missing_provider_1",
		Message: im.messages(conversationID)[0],
	}); err != nil {
		t.Fatalf("handle should not be blocked by missing provider, got: %v", err)
	}

	run := waitForAgentRunStatus(t, auditStore, "run_hosted_1", agentaudit.StatusFailed)
	if !strings.Contains(run.ErrorMessage, missingProviderErr.Error()) {
		t.Fatalf("agent run error = %q, want missing provider", run.ErrorMessage)
	}

	messages := waitForConversationCount(t, im, conversationID, 2)
	if strings.Contains(messages[0].Content, "AI reply") {
		t.Fatalf("human message content was replaced by fake text: %+v", messages[0])
	}
	failureNotice := messages[1]
	if failureNotice.MessageOrigin != MessageOriginAI || failureNotice.AgentAccountID != "usr_a" || failureNotice.TriggerServerMsgID == "" {
		t.Fatalf("failure notice metadata mismatch: %+v", failureNotice)
	}
	if !strings.Contains(failureNotice.Content, "AI 助手这次处理失败") || !strings.Contains(failureNotice.Content, "模型或工具权限配置不可用") {
		t.Fatalf("failure notice content = %q, want user-readable provider failure", failureNotice.Content)
	}
	if strings.Contains(failureNotice.Content, "DEEPSEEK_API_KEY") || strings.Contains(failureNotice.Content, missingProviderErr.Error()) {
		t.Fatalf("failure notice leaked internal provider configuration: %q", failureNotice.Content)
	}
}

func waitForAgentRunStatus(t *testing.T, repo *agaudit.MemoryStore, runID string, status agentaudit.Status) agentaudit.AgentRun {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		run, err := repo.GetAgentRun(context.Background(), runID)
		if err == nil && run.Status == status {
			return run
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("load agent run %q waiting for status %q: %v", runID, status, err)
			}
			t.Fatalf("timed out waiting for agent run %q status %q", runID, status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
