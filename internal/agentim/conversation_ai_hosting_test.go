package agentim

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/internal/config"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestConversationAIHostingPeerHumanMessageWritesAIReplyAsHostedOwner(t *testing.T) {
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
	runtime := agentruntime.RuntimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		if req.AgentUserID != "usr_a" || req.RequestingUserID != "usr_b" {
			t.Fatalf("runtime request used wrong hosting owner/requester: %+v", req)
		}
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

	trigger, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_b",
		ReceiverID:  "usr_a",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "human-peer-trigger",
		ContentType: logic.MessageContentTypeText,
		Content:     "你好",
	})
	if err != nil {
		t.Fatalf("send peer human trigger: %v", err)
	}
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
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
	if len(pulled.Messages) != 2 {
		t.Fatalf("messages = %+v, want human + ai reply", pulled.Messages)
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

func TestConversationAIHostingMissingProviderFailsWithoutFakeAIMessage(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	hostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	aiHostingRepo := repository.NewMemoryConversationAIHostingRepository()
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
		Audit:  logic.NewAgentAuditLogic(repository.NewMemoryAgentAuditRepository()),
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository:          hostingRepo,
		AIHostingRepository: aiHostingRepo,
		Runner:              orchestrator,
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
	if !errors.Is(err, missingProviderErr) {
		t.Fatalf("send error = %v, want missing provider error", err)
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
	if len(pulled.Messages) != 1 {
		t.Fatalf("provider failure created fake AI message: %+v", pulled.Messages)
	}
	if strings.Contains(pulled.Messages[0].Content, "AI reply") {
		t.Fatalf("human message content was replaced by fake text: %+v", pulled.Messages[0])
	}
}
