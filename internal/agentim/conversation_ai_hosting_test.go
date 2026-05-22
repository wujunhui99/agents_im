package agentim

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/model"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestPrivateAgentChatTriggersAgentReply(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	hostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	agentRepo := repository.NewMemoryAgentRepository()
	auditRepo := repository.NewMemoryAgentAuditRepository()
	writer, err := NewMessageServiceResponseWriter(messageLogic)
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
	runtime := agentruntime.RuntimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
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
		RequestBuilder: RuntimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  logic.NewAgentAuditLogic(auditRepo),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:           hostingRepo,
		AIHostingRepository:  aiHostingRepo,
		Runner:               orchestrator,
		AgentAccountResolver: NewAgentRepositoryAccountResolver(agentRepo),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}
	messageLogic.SetMessageCreatedHook(hosting)

	conversationID := repository.SingleConversationID("usr_new", "agent_creator")
	trigger, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_new",
		ReceiverID:  "agent_creator",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "ask-agent-creator",
		ContentType: logic.MessageContentTypeText,
		Content:     "你是谁？",
	})
	if err != nil {
		t.Fatalf("send private agent message: %v", err)
	}

	pulled := waitForPulledMessageCount(t, messageLogic, "usr_new", conversationID, 2)
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	reply := pulled.Messages[1]
	if reply.MessageOrigin != logic.MessageOriginAI || reply.SenderID != "agent_creator" || reply.AgentAccountID != "agent_creator" {
		t.Fatalf("reply did not use agent_creator ai metadata: %+v", reply)
	}
	if reply.ReceiverID != "usr_new" || reply.TriggerServerMsgID != trigger.Message.ServerMsgID {
		t.Fatalf("reply routing/trigger metadata mismatch: trigger=%+v reply=%+v", trigger.Message, reply)
	}
}

func TestConversationAIHostingSlowGenerationDoesNotBlockSendAndMarksReadFirst(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	hostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	auditRepo := repository.NewMemoryAgentAuditRepository()
	writer, err := NewMessageServiceResponseWriter(messageLogic)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	runtimeCalls := 0
	runtimeStarted := make(chan struct{})
	releaseRuntime := make(chan struct{})
	runtime := agentruntime.RuntimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		if runtimeCalls == 1 {
			close(runtimeStarted)
		}
		if req.AgentUserID != "usr_a" || req.RequestingUserID != "usr_b" {
			t.Fatalf("runtime request used wrong hosting owner/requester: %+v", req)
		}
		<-releaseRuntime
		return agentruntime.RunResult{
			RunID:     req.RunID,
			FinalText: "托管回复",
		}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: RuntimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  logic.NewAgentAuditLogic(auditRepo),
		Writer: writer,
		Now: func() time.Time {
			return time.Unix(200, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:          hostingRepo,
		AIHostingRepository: aiHostingRepo,
		Runner:              orchestrator,
		ReadMarker:          NewMessageRepositoryReadMarker(messageRepo),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}
	messageLogic.SetMessageCreatedHook(hosting)

	conversationID := repository.SingleConversationID("usr_a", "usr_b")
	if _, err := aiHostingRepo.SetConversationAIHostingEnabled(ctx, repository.ConversationAIHostingUpdate{
		OwnerAccountID:    "usr_a",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable AI hosting: %v", err)
	}

	type sendResult struct {
		resp logic.SendMessageResponse
		err  error
	}
	sendDone := make(chan sendResult, 1)
	go func() {
		resp, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
			SenderID:    "usr_b",
			ReceiverID:  "usr_a",
			ChatType:    logic.MessageChatTypeSingle,
			ClientMsgID: "human-peer-trigger",
			ContentType: logic.MessageContentTypeText,
			Content:     "你好",
		})
		sendDone <- sendResult{resp: resp, err: err}
	}()

	select {
	case <-runtimeStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runtime did not start")
	}

	var trigger logic.SendMessageResponse
	select {
	case result := <-sendDone:
		if result.err != nil {
			close(releaseRuntime)
			t.Fatalf("send peer human trigger: %v", result.err)
		}
		trigger = result.resp
	case <-time.After(50 * time.Millisecond):
		close(releaseRuntime)
		result := <-sendDone
		if result.err != nil {
			t.Fatalf("send waited for AI generation and then failed: %v", result.err)
		}
		t.Fatalf("send waited for AI generation; response arrived only after release: %+v", result.resp.Message)
	}

	ownerSeqs, err := messageLogic.GetConversationSeqs(ctx, logic.GetConversationSeqsRequest{
		UserID:          "usr_a",
		ConversationIDs: []string{conversationID},
	})
	if err != nil {
		close(releaseRuntime)
		t.Fatalf("get owner conversation seqs before AI completion: %v", err)
	}
	if len(ownerSeqs.States) != 1 || ownerSeqs.States[0].HasReadSeq != trigger.Message.Seq || ownerSeqs.States[0].UnreadCount != 0 {
		close(releaseRuntime)
		t.Fatalf("hosted owner read state before AI completion = %+v, want hasReadSeq %d unread 0", ownerSeqs.States, trigger.Message.Seq)
	}

	pulled, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_b",
		ConversationID: conversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull messages: %v", err)
	}
	if len(pulled.Messages) != 1 {
		close(releaseRuntime)
		t.Fatalf("messages before AI completion = %+v, want only human trigger", pulled.Messages)
	}

	close(releaseRuntime)
	pulled = waitForPulledMessageCount(t, messageLogic, "usr_b", conversationID, 2)

	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	reply := pulled.Messages[1]
	if reply.MessageOrigin != logic.MessageOriginAI || reply.SenderID != "usr_a" || reply.AgentAccountID != "usr_a" {
		t.Fatalf("reply did not use hosted owner ai metadata: %+v", reply)
	}
	if reply.TriggerServerMsgID != trigger.Message.ServerMsgID {
		t.Fatalf("reply trigger metadata = %q, want %q", reply.TriggerServerMsgID, trigger.Message.ServerMsgID)
	}
	if runtimeCalls != 1 {
		t.Fatalf("ai reply recursively triggered runtime: %d", runtimeCalls)
	}
}

func TestConversationAIHostingDuplicateTriggerDoesNotQueueDuplicateReply(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	hostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	auditRepo := repository.NewMemoryAgentAuditRepository()
	writer, err := NewMessageServiceResponseWriter(messageLogic)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}

	runtimeCalls := 0
	releaseRuntime := make(chan struct{})
	runtime := agentruntime.RuntimeFunc(func(context.Context, agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		<-releaseRuntime
		return agentruntime.RunResult{RunID: "run_hosted_1", FinalText: "托管回复"}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: RuntimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  logic.NewAgentAuditLogic(auditRepo),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:          hostingRepo,
		AIHostingRepository: aiHostingRepo,
		Runner:              orchestrator,
		ReadMarker:          NewMessageRepositoryReadMarker(messageRepo),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}
	messageLogic.SetMessageCreatedHook(hosting)

	conversationID := repository.SingleConversationID("usr_a", "usr_b")
	if _, err := aiHostingRepo.SetConversationAIHostingEnabled(ctx, repository.ConversationAIHostingUpdate{
		OwnerAccountID:    "usr_a",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable AI hosting: %v", err)
	}

	first, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_b",
		ReceiverID:  "usr_a",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "human-peer-trigger",
		ContentType: logic.MessageContentTypeText,
		Content:     "你好",
	})
	if err != nil {
		close(releaseRuntime)
		t.Fatalf("send first trigger: %v", err)
	}
	duplicate, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_b",
		ReceiverID:  "usr_a",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "human-peer-trigger",
		ContentType: logic.MessageContentTypeText,
		Content:     "你好",
	})
	if err != nil {
		close(releaseRuntime)
		t.Fatalf("send duplicate trigger: %v", err)
	}
	if !duplicate.Deduplicated || duplicate.Message.ServerMsgID != first.Message.ServerMsgID {
		close(releaseRuntime)
		t.Fatalf("duplicate send mismatch: first=%+v duplicate=%+v", first, duplicate)
	}

	close(releaseRuntime)
	pulled := waitForPulledMessageCount(t, messageLogic, "usr_b", conversationID, 2)
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	aiReplies := 0
	for _, message := range pulled.Messages {
		if message.MessageOrigin == logic.MessageOriginAI {
			aiReplies++
		}
	}
	if aiReplies != 1 {
		t.Fatalf("ai replies = %d, messages=%+v", aiReplies, pulled.Messages)
	}
}

func TestConversationAIHostingMissingProviderDoesNotBlockOriginalSendAndNotifiesUser(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	hostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
	auditRepo := repository.NewMemoryAgentAuditRepository()
	writer, err := NewMessageServiceResponseWriter(messageLogic)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}
	missingProviderErr := config.ErrDeepSeekAPIKeyMissing
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: agentruntime.RuntimeFunc(func(context.Context, agentruntime.RunRequest) (agentruntime.RunResult, error) {
			return agentruntime.RunResult{}, missingProviderErr
		}),
		RequestBuilder: RuntimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  logic.NewAgentAuditLogic(auditRepo),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:          hostingRepo,
		AIHostingRepository: aiHostingRepo,
		Runner:              orchestrator,
		ReadMarker:          NewMessageRepositoryReadMarker(messageRepo),
	})
	if err != nil {
		t.Fatalf("new hosting: %v", err)
	}
	messageLogic.SetMessageCreatedHook(hosting)

	conversationID := repository.SingleConversationID("usr_a", "usr_b")
	if _, err := aiHostingRepo.SetConversationAIHostingEnabled(ctx, repository.ConversationAIHostingUpdate{
		OwnerAccountID:    "usr_a",
		ConversationID:    conversationID,
		Enabled:           true,
		MaxRecentMessages: 30,
	}); err != nil {
		t.Fatalf("enable AI hosting: %v", err)
	}

	_, err = messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_b",
		ReceiverID:  "usr_a",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "human-trigger-missing-provider",
		ContentType: logic.MessageContentTypeText,
		Content:     "需要托管回复",
	})
	if err != nil {
		t.Fatalf("send should not be blocked by missing provider, got: %v", err)
	}

	run := waitForAgentRunStatus(t, auditRepo, "run_hosted_1", agentaudit.StatusFailed)
	if !strings.Contains(run.ErrorMessage, missingProviderErr.Error()) {
		t.Fatalf("agent run error = %q, want missing provider", run.ErrorMessage)
	}

	pulled, pullErr := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_b",
		ConversationID: conversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          "asc",
	})
	if pullErr != nil {
		t.Fatalf("pull after provider failure: %v", pullErr)
	}
	if len(pulled.Messages) != 2 {
		t.Fatalf("provider failure messages = %+v, want original human message plus user-visible failure notice", pulled.Messages)
	}
	if strings.Contains(pulled.Messages[0].Content, "AI reply") {
		t.Fatalf("human message content was replaced by fake text: %+v", pulled.Messages[0])
	}
	failureNotice := pulled.Messages[1]
	if failureNotice.MessageOrigin != logic.MessageOriginAI || failureNotice.AgentAccountID != "usr_a" || failureNotice.TriggerServerMsgID == "" {
		t.Fatalf("failure notice metadata mismatch: %+v", failureNotice)
	}
	if !strings.Contains(failureNotice.Content, "AI 助手这次处理失败") || !strings.Contains(failureNotice.Content, "模型或工具权限配置不可用") {
		t.Fatalf("failure notice content = %q, want user-readable provider failure", failureNotice.Content)
	}
	if strings.Contains(failureNotice.Content, "DEEPSEEK_API_KEY") || strings.Contains(failureNotice.Content, missingProviderErr.Error()) {
		t.Fatalf("failure notice leaked internal provider configuration: %q", failureNotice.Content)
	}
}

func waitForPulledMessageCount(t *testing.T, messageLogic *logic.MessageLogic, userID string, conversationID string, want int) logic.PullMessagesResponse {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		pulled, err := messageLogic.PullMessages(context.Background(), logic.PullMessagesRequest{
			UserID:         userID,
			ConversationID: conversationID,
			FromSeq:        1,
			Limit:          10,
			Order:          "asc",
		})
		if err == nil && len(pulled.Messages) == want {
			return pulled
		}
		if time.Now().After(deadline) {
			if err != nil {
				t.Fatalf("pull messages waiting for %d messages: %v", want, err)
			}
			t.Fatalf("timed out waiting for %d messages", want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForAgentRunStatus(t *testing.T, repo *repository.MemoryAgentAuditRepository, runID string, status agentaudit.Status) agentaudit.AgentRun {
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
