package agentim

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/common/share/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/llmobs"
	"github.com/wujunhui99/agents_im/internal/logic"
)

func TestAgentRunOrchestratorEmitsLLMObservabilityMetadata(t *testing.T) {
	sink := llmobs.NewMemorySink()
	trigger := testAgentTrigger()
	runtimeReq := validRuntimeRequestForTrigger(trigger)
	runtimeReq.RunID = "run_obs_1"
	runtimeReq.Agent.Prompt.Version = "prompt-v1"
	runtimeReq.Metadata = map[string]string{
		"runtime_mode":         llmobs.RuntimeModeAIHostingAutoReply,
		"recent_message_count": "1",
	}
	runtime := &recordingAgentRuntime{
		result: agentruntime.RunResult{
			RunID:        "run_obs_1",
			FinalText:    "Go is usually faster for CPU-bound services.",
			FinishReason: "stop",
			Model: agentruntime.ModelMetadata{
				Provider: "deepseek",
				Model:    "deepseek-v4-pro",
			},
			Usage: agentruntime.Usage{
				PromptTokens:     12,
				CompletionTokens: 8,
				TotalTokens:      20,
			},
		},
	}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_obs_1"},
	}
	writer := &recordingAgentResponseWriter{
		result: AgentResponseResult{
			Message: logic.Message{
				ServerMsgID:    "msg_agent_obs_1",
				ConversationID: trigger.ConversationID,
				Seq:            8,
				SenderID:       trigger.AgentUserID,
				ReceiverID:     trigger.RequestingUserID,
				ChatType:       ConversationTypeSingle,
				ContentType:    ContentTypeText,
				Content:        "Go is usually faster for CPU-bound services.",
			},
			Metadata: AgentMessageMetadata{
				AgentRunID:       "run_obs_1",
				TriggerMessageID: trigger.TriggerMessageID,
			},
		},
	}
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime:              runtime,
		RequestBuilder:       &recordingRuntimeRequestBuilder{request: runtimeReq},
		Audit:                audit,
		Writer:               writer,
		LLMObservabilitySink: sink,
		Now: func() time.Time {
			return time.Unix(100, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}

	if _, err := orchestrator.Run(context.Background(), trigger); err != nil {
		t.Fatalf("run orchestrator: %v", err)
	}

	events := sink.Events()
	if len(events) != 2 {
		t.Fatalf("observability events = %d, want started and succeeded: %+v", len(events), events)
	}
	started, succeeded := events[0], events[1]
	if started.Status != llmobs.StatusStarted || succeeded.Status != llmobs.StatusSucceeded {
		t.Fatalf("statuses = %q/%q, want started/succeeded", started.Status, succeeded.Status)
	}
	if succeeded.TraceID != "trace_1" || succeeded.RequestID != "evt_1:agent_1" ||
		succeeded.AgentRunID != "run_obs_1" || succeeded.ConversationID != "single:agent_1:user_1" ||
		succeeded.TriggerServerMsgID != "msg_user_1" || succeeded.ResponseServerMsgID != "msg_agent_obs_1" {
		t.Fatalf("succeeded event did not preserve run linkage: %+v", succeeded)
	}
	if succeeded.HostedOwnerAccountID != "agent_1" || succeeded.SenderAccountID != "user_1" || succeeded.AgentAccountID != "agent_1" {
		t.Fatalf("actor metadata mismatch: %+v", succeeded)
	}
	if succeeded.ModelProvider != "deepseek" || succeeded.ModelName != "deepseek-v4-pro" || succeeded.PromptVersion != "prompt-v1" {
		t.Fatalf("model/prompt metadata mismatch: %+v", succeeded)
	}
	if succeeded.RuntimeMode != llmobs.RuntimeModeAIHostingAutoReply || succeeded.LatencyMs != 0 {
		t.Fatalf("runtime mode/latency mismatch: %+v", succeeded)
	}
	if succeeded.Generation.BoundedRecentMessageCount != 1 || !succeeded.Generation.TriggerInContext {
		t.Fatalf("generation context metadata mismatch: %+v", succeeded.Generation)
	}
	if succeeded.Generation.TotalTokens != 20 || succeeded.Generation.FinishReason != "stop" {
		t.Fatalf("usage metadata mismatch: %+v", succeeded.Generation)
	}
}

func TestAgentRunOrchestratorEmitsLLMObservabilityFailure(t *testing.T) {
	sink := llmobs.NewMemorySink()
	trigger := testAgentTrigger()
	runtimeReq := validRuntimeRequestForTrigger(trigger)
	runtimeReq.RunID = "run_obs_failed_1"
	runtimeReq.Metadata = map[string]string{"runtime_mode": llmobs.RuntimeModeAIHostingAutoReply}
	runtimeErr := errors.New("provider failed with token=secret-value")
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime:              &recordingAgentRuntime{err: runtimeErr},
		RequestBuilder:       &recordingRuntimeRequestBuilder{request: runtimeReq},
		Audit:                &recordingAgentRunAuditRecorder{run: agentaudit.AgentRun{RunID: "run_obs_failed_1"}},
		Writer:               &recordingAgentResponseWriter{},
		LLMObservabilitySink: sink,
		Now: func() time.Time {
			return time.Unix(100, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}

	_, err = orchestrator.Run(context.Background(), trigger)
	if !errors.Is(err, runtimeErr) {
		t.Fatalf("run error = %v, want runtime error", err)
	}

	events := sink.Events()
	if len(events) != 2 {
		t.Fatalf("observability events = %d, want started and failed: %+v", len(events), events)
	}
	failed := events[1]
	if failed.Status != llmobs.StatusFailed || failed.AgentRunID != "run_obs_failed_1" {
		t.Fatalf("failed event mismatch: %+v", failed)
	}
	if failed.ResponseServerMsgID != "" {
		t.Fatalf("failed event must not imply a response export/writeback succeeded: %+v", failed)
	}
	if failed.ErrorClass == "" || failed.ErrorMessage == "" {
		t.Fatalf("failed event should include sanitized error metadata: %+v", failed)
	}
	if failed.ErrorMessage == runtimeErr.Error() || failed.ErrorMessage == "provider failed with token=secret-value" {
		t.Fatalf("failed event leaked unsanitized error: %+v", failed)
	}
}
