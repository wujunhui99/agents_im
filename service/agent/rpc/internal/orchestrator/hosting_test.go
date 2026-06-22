package orchestrator

import (
	"context"
	"testing"
	"time"

	agentruntime "github.com/wujunhui99/agents_im/service/agent/rpc/internal/runtime"
	"github.com/wujunhui99/agents_im/internal/logic"
	"github.com/wujunhui99/agents_im/internal/repository"
	"github.com/wujunhui99/agents_im/pkg/agentaudit"
)

func TestConversationHostingWritesAIResponseThroughMessageServiceAndDeduplicates(t *testing.T) {
	ctx := context.Background()
	messageRepo := repository.NewMemoryMessageRepository()
	messageLogic := logic.NewMessageLogic(messageRepo)
	hostingRepo := repository.NewMemoryAgentConversationHostingRepository()
	auditRepo := repository.NewMemoryAgentAuditRepository()
	auditLogic := logic.NewAgentAuditLogic(auditRepo)
	writer, err := NewMessageServiceResponseWriter(messageLogic)
	if err != nil {
		t.Fatalf("new response writer: %v", err)
	}
	runtimeCalls := 0
	runtime := agentruntime.RuntimeFunc(func(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
		runtimeCalls++
		return agentruntime.RunResult{
			RunID:     "run_hosted_1",
			FinalText: "AI reply to: " + req.PromptText,
		}, nil
	})
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		RequestBuilder: RuntimeRequestBuilderFunc(func(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
			return hostedRuntimeRequest(trigger), nil
		}),
		Audit:  auditLogic,
		Writer: writer,
		Now: func() time.Time {
			return time.Unix(200, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	hosting, err := NewConversationHostingService(ConversationHostingConfig{
		Repository: hostingRepo,
		Runner:     orchestrator,
	})
	if err != nil {
		t.Fatalf("new hosting service: %v", err)
	}
	if _, err := hostingRepo.UpsertAgentConversationHosting(ctx, repository.AgentConversationHosting{
		ConversationID: repository.SingleConversationID("usr_1", "agent_1"),
		AgentAccountID: "agent_1",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("upsert hosting config: %v", err)
	}
	messageLogic.SetMessageCreatedHook(hosting)

	human, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_1",
		ReceiverID:  "agent_1",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "client-hosting-human",
		ContentType: logic.MessageContentTypeText,
		Content:     "hello",
	})
	if err != nil {
		t.Fatalf("send human trigger: %v", err)
	}

	pulled := waitForPulledMessageCount(t, messageLogic, "usr_1", human.Message.ConversationID, 2)
	if runtimeCalls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtimeCalls)
	}
	if len(pulled.Messages) != 2 {
		t.Fatalf("got %d messages, want human + ai: %+v", len(pulled.Messages), pulled.Messages)
	}
	aiMessage := pulled.Messages[1]
	if aiMessage.MessageOrigin != logic.MessageOriginAI {
		t.Fatalf("expected ai response message: %+v", aiMessage)
	}
	if aiMessage.AgentAccountID != "agent_1" ||
		aiMessage.TriggerServerMsgID != human.Message.ServerMsgID {
		t.Fatalf("ai response did not preserve agent trigger metadata: %+v", aiMessage)
	}

	duplicate, err := messageLogic.SendMessage(ctx, logic.SendMessageRequest{
		SenderID:    "usr_1",
		ReceiverID:  "agent_1",
		ChatType:    logic.MessageChatTypeSingle,
		ClientMsgID: "client-hosting-human",
		ContentType: logic.MessageContentTypeText,
		Content:     "hello",
	})
	if err != nil {
		t.Fatalf("send duplicate human trigger: %v", err)
	}
	if !duplicate.Deduplicated {
		t.Fatalf("duplicate message was not deduplicated: %+v", duplicate)
	}
	if runtimeCalls != 1 {
		t.Fatalf("duplicate trigger called runtime: %d", runtimeCalls)
	}

	afterDuplicate, err := messageLogic.PullMessages(ctx, logic.PullMessagesRequest{
		UserID:         "usr_1",
		ConversationID: human.Message.ConversationID,
		FromSeq:        1,
		Limit:          10,
		Order:          "asc",
	})
	if err != nil {
		t.Fatalf("pull hosted conversation after duplicate: %v", err)
	}
	if len(afterDuplicate.Messages) != 2 {
		t.Fatalf("duplicate trigger produced another response: %+v", afterDuplicate.Messages)
	}

	aiResult, err := hosting.HandleMessageCreated(ctx, ConversationHostingMessageCreatedInput{
		EventID: "evt_ai_1",
		Message: aiMessage,
	})
	if err != nil {
		t.Fatalf("handle ai message: %v", err)
	}
	if aiResult.Triggered {
		t.Fatalf("ai-origin message should not trigger by default: %+v", aiResult)
	}

	run, err := auditRepo.GetAgentRun(ctx, "run_hosted_1")
	if err != nil {
		t.Fatalf("load agent run audit: %v", err)
	}
	if run.Status != agentaudit.StatusSucceeded || run.TriggerMessageID != human.Message.ServerMsgID {
		t.Fatalf("agent audit mismatch: %+v", run)
	}
}

func hostedRuntimeRequest(trigger AgentTrigger) agentruntime.RunRequest {
	return agentruntime.RunRequest{
		RunID:              "run_hosted_1",
		RequestID:          trigger.RequestID,
		EventID:            trigger.EventID,
		OperationID:        trigger.OperationID,
		TraceID:            trigger.TraceID,
		TriggerType:        trigger.TriggerType,
		AgentUserID:        trigger.AgentUserID,
		RequestingUserID:   trigger.RequestingUserID,
		ConversationID:     trigger.ConversationID,
		ConversationType:   trigger.ConversationType,
		TriggerMessageID:   trigger.TriggerMessageID,
		TriggerSeq:         trigger.TriggerSeq,
		PromptText:         trigger.PromptText,
		ReplyToMessageID:   trigger.ReplyToMessageID,
		SourceMessageID:    trigger.SourceMessageID,
		SourceMessageSeq:   trigger.SourceMessageSeq,
		SourceMessageText:  trigger.SourceMessageText,
		SourceContentType:  trigger.SourceContentType,
		TargetAgentUserIDs: append([]string(nil), trigger.TargetAgentUserIDs...),
		Agent: agentruntime.AgentConfig{
			AgentID:     "agent_profile_1",
			AgentUserID: trigger.AgentUserID,
			Name:        "Hosted Agent",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "prompt_1",
				Content:  "Use the deterministic test runtime.",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deterministic-test",
				Model:    "deterministic-v1",
			},
			Policy: agentruntime.RuntimePolicy{
				RequireMessageServiceWriteback: true,
			},
		},
		Conversation: []agentruntime.ConversationMessage{{
			ServerMsgID: trigger.SourceMessageID,
			Seq:         trigger.SourceMessageSeq,
			SenderID:    trigger.RequestingUserID,
			SenderType:  agentruntime.SenderTypeUser,
			ContentType: trigger.SourceContentType,
			Text:        trigger.SourceMessageText,
		}},
	}
}
