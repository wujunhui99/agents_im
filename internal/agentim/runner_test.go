package agentim

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/agentruntime"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/logic"
)

func TestAgentRunOrchestratorSuccessWritesAuditedResponse(t *testing.T) {
	runtime := &recordingAgentRuntime{
		result: agentruntime.RunResult{
			RunID:     "run_1",
			FinalText: " answer ",
			Model: agentruntime.ModelMetadata{
				Provider: "deepseek",
				Model:    "test-model",
			},
			Metadata: map[string]string{"allow_recursive_trigger": "true"},
		},
	}
	runtimeReq := validRuntimeRequestForTrigger(testAgentTrigger())
	runtimeReq.Agent.Policy.AllowAgentMessageRecursion = true
	builder := &recordingRuntimeRequestBuilder{request: runtimeReq}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_1"},
	}
	writer := &recordingAgentResponseWriter{
		result: AgentResponseResult{
			Message: logic.Message{
				ServerMsgID:    "msg_agent_1",
				ConversationID: "single:agent_1:user_1",
				Seq:            9,
				SenderID:       "agent_1",
				ReceiverID:     "user_1",
				ChatType:       ConversationTypeSingle,
				ContentType:    ContentTypeText,
				Content:        "answer",
			},
			Metadata: AgentMessageMetadata{
				AgentRunID:            "run_1",
				TriggerMessageID:      "msg_user_1",
				AllowRecursiveTrigger: true,
			},
		},
	}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	result, err := orchestrator.Run(context.Background(), testAgentTrigger())
	if err != nil {
		t.Fatalf("run orchestrator: %v", err)
	}
	if result.AuditRun.RunID != "run_1" || result.Response.Message.ServerMsgID != "msg_agent_1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if runtime.calls != 1 {
		t.Fatalf("runtime calls = %d, want 1", runtime.calls)
	}
	if runtime.lastReq.RequestID != "evt_1:agent_1" {
		t.Fatalf("runtime did not receive normalized request: %+v", runtime.lastReq)
	}
	if builder.calls != 1 || builder.lastTrigger.RequestID != "evt_1:agent_1" {
		t.Fatalf("builder did not receive normalized trigger: calls=%d trigger=%+v", builder.calls, builder.lastTrigger)
	}
	if audit.calls != 1 {
		t.Fatalf("audit calls = %d, want 1", audit.calls)
	}
	if audit.lastInput.Status != agentaudit.StatusSucceeded {
		t.Fatalf("audit status = %s, want %s", audit.lastInput.Status, agentaudit.StatusSucceeded)
	}
	if audit.lastInput.AgentID != "agent_1" || audit.lastInput.TriggerMessageID != "msg_user_1" {
		t.Fatalf("audit input did not preserve trigger metadata: %+v", audit.lastInput)
	}
	modelSummary, ok := audit.lastInput.OutputSummary["model"].(agentaudit.Summary)
	if !ok || audit.lastInput.OutputSummary["final_text"] != "answer" || modelSummary["model"] != "test-model" {
		t.Fatalf("audit output summary mismatch: %+v", audit.lastInput.OutputSummary)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}
	if writer.lastReq.AgentRunID != "run_1" || writer.lastReq.TriggerMessageID != "msg_user_1" {
		t.Fatalf("response request did not preserve loop metadata: %+v", writer.lastReq)
	}
	if !writer.lastReq.AllowRecursiveTrigger {
		t.Fatalf("allow_recursive_trigger was not forwarded: %+v", writer.lastReq)
	}
	if writer.lastReq.ReceiverUserID != "user_1" || writer.lastReq.GroupID != "" {
		t.Fatalf("response target mismatch: %+v", writer.lastReq)
	}
	if writer.lastReq.RequestID != "evt_1:agent_1:response" {
		t.Fatalf("response request id = %q", writer.lastReq.RequestID)
	}
}

func TestAgentRunOrchestratorRuntimeFailureRecordsFailedAuditAndNotifiesUser(t *testing.T) {
	runtimeErr := errors.New("deepseek generate AI hosting reply: NOT_FOUND: tool not found")
	runtime := &recordingAgentRuntime{err: runtimeErr}
	builder := &recordingRuntimeRequestBuilder{request: validRuntimeRequestForTrigger(testAgentTrigger())}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_failed_1"},
	}
	writer := &recordingAgentResponseWriter{
		result: AgentResponseResult{
			Message: logic.Message{
				ServerMsgID:    "msg_failure_notice_1",
				ConversationID: "single:agent_1:user_1",
				Seq:            9,
				SenderID:       "agent_1",
				ReceiverID:     "user_1",
				ChatType:       ConversationTypeSingle,
				ContentType:    ContentTypeText,
				Content:        "抱歉，AI 助手这次处理失败了。错误类型：工具配置不可用。",
			},
		},
	}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	_, err := orchestrator.Run(context.Background(), testAgentTrigger())
	if !errors.Is(err, runtimeErr) {
		t.Fatalf("got error %v, want runtime error", err)
	}
	if audit.calls != 1 {
		t.Fatalf("audit calls = %d, want 1", audit.calls)
	}
	if audit.lastInput.Status != agentaudit.StatusFailed || audit.lastInput.ErrorCode != "runtime_error" {
		t.Fatalf("audit did not record runtime failure: %+v", audit.lastInput)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want user-visible failure notice", writer.calls)
	}
	if writer.lastReq.AgentRunID != "run_failed_1" || writer.lastReq.TriggerMessageID != "msg_user_1" {
		t.Fatalf("failure notice metadata mismatch: %+v", writer.lastReq)
	}
	if writer.lastReq.ReceiverUserID != "user_1" || writer.lastReq.GroupID != "" {
		t.Fatalf("failure notice target mismatch: %+v", writer.lastReq)
	}
	if !strings.Contains(writer.lastReq.Text, "AI 助手这次处理失败") || !strings.Contains(writer.lastReq.Text, "工具配置不可用") {
		t.Fatalf("failure notice text = %q, want user-readable tool config failure", writer.lastReq.Text)
	}
	if strings.Contains(writer.lastReq.Text, "NOT_FOUND") || strings.Contains(writer.lastReq.Text, "tool not found") || strings.Contains(writer.lastReq.Text, "deepseek") {
		t.Fatalf("failure notice leaked internal error: %q", writer.lastReq.Text)
	}
}

func TestAgentRunOrchestratorRequiresPolicyForRecursiveResponse(t *testing.T) {
	runtime := &recordingAgentRuntime{
		result: agentruntime.RunResult{
			RunID:     "run_1",
			FinalText: "answer",
			Metadata:  map[string]string{"allow_recursive_trigger": "true"},
		},
	}
	builder := &recordingRuntimeRequestBuilder{request: validRuntimeRequestForTrigger(testAgentTrigger())}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_1"},
	}
	writer := &recordingAgentResponseWriter{
		result: AgentResponseResult{
			Message: logic.Message{
				ServerMsgID:    "msg_agent_1",
				ConversationID: "single:agent_1:user_1",
				Seq:            9,
				SenderID:       "agent_1",
				ReceiverID:     "user_1",
				ChatType:       ConversationTypeSingle,
				ContentType:    ContentTypeText,
				Content:        "answer",
			},
		},
	}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	if _, err := orchestrator.Run(context.Background(), testAgentTrigger()); err != nil {
		t.Fatalf("run orchestrator: %v", err)
	}
	if writer.lastReq.AllowRecursiveTrigger {
		t.Fatalf("allow_recursive_trigger should require runtime policy opt-in: %+v", writer.lastReq)
	}
}

func TestAgentRunOrchestratorRequestBuilderFailureRecordsFailedAudit(t *testing.T) {
	builderErr := errors.New("agent config unavailable")
	runtime := &recordingAgentRuntime{result: agentruntime.RunResult{RunID: "run_1", FinalText: "answer"}}
	builder := &recordingRuntimeRequestBuilder{err: builderErr}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_request_failed_1"},
	}
	writer := &recordingAgentResponseWriter{}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	_, err := orchestrator.Run(context.Background(), testAgentTrigger())
	if !errors.Is(err, builderErr) {
		t.Fatalf("got error %v, want builder error", err)
	}
	if runtime.calls != 0 {
		t.Fatalf("runtime calls = %d, want 0", runtime.calls)
	}
	if audit.calls != 1 || audit.lastInput.Status != agentaudit.StatusFailed || audit.lastInput.ErrorCode != "runtime_request_error" {
		t.Fatalf("audit did not record request build failure: calls=%d input=%+v", audit.calls, audit.lastInput)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want user-visible failure notice", writer.calls)
	}
	if !strings.Contains(writer.lastReq.Text, "AI 助手这次处理失败") {
		t.Fatalf("failure notice text = %q, want user-readable notice", writer.lastReq.Text)
	}
}

func TestAgentRunOrchestratorAuditFailureStopsResponseWrite(t *testing.T) {
	auditErr := errors.New("audit database unavailable")
	runtime := &recordingAgentRuntime{result: agentruntime.RunResult{RunID: "run_1", FinalText: "answer"}}
	builder := &recordingRuntimeRequestBuilder{request: validRuntimeRequestForTrigger(testAgentTrigger())}
	audit := &recordingAgentRunAuditRecorder{err: auditErr}
	writer := &recordingAgentResponseWriter{}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	_, err := orchestrator.Run(context.Background(), testAgentTrigger())
	if !errors.Is(err, auditErr) {
		t.Fatalf("got error %v, want audit error", err)
	}
	if audit.calls != 1 {
		t.Fatalf("audit calls = %d, want 1", audit.calls)
	}
	if writer.calls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.calls)
	}
}

func TestAgentRunOrchestratorRejectsEmptyFinalText(t *testing.T) {
	runtime := &recordingAgentRuntime{result: agentruntime.RunResult{RunID: "run_1", FinalText: " \t\n "}}
	builder := &recordingRuntimeRequestBuilder{request: validRuntimeRequestForTrigger(testAgentTrigger())}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_empty_1"},
	}
	writer := &recordingAgentResponseWriter{}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	_, err := orchestrator.Run(context.Background(), testAgentTrigger())
	if err == nil {
		t.Fatal("expected empty final text error")
	}
	if apperror.From(err).Code != apperror.CodeInvalidArgument {
		t.Fatalf("error code = %s, want %s", apperror.From(err).Code, apperror.CodeInvalidArgument)
	}
	if audit.calls != 1 {
		t.Fatalf("audit calls = %d, want 1", audit.calls)
	}
	if audit.lastInput.Status != agentaudit.StatusFailed || audit.lastInput.ErrorCode != "empty_final_text" {
		t.Fatalf("audit did not record empty final text rejection: %+v", audit.lastInput)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want user-visible failure notice", writer.calls)
	}
	if !strings.Contains(writer.lastReq.Text, "AI 助手这次处理失败") {
		t.Fatalf("failure notice text = %q, want user-readable notice", writer.lastReq.Text)
	}
}

func TestAgentRunOrchestratorPropagatesResponseWriterFailure(t *testing.T) {
	writerErr := errors.New("message service rejected response")
	runtime := &recordingAgentRuntime{result: agentruntime.RunResult{RunID: "run_1", FinalText: "answer"}}
	builder := &recordingRuntimeRequestBuilder{request: validRuntimeRequestForTrigger(testAgentTrigger())}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_1"},
	}
	writer := &recordingAgentResponseWriter{err: writerErr}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, builder, audit, writer)

	_, err := orchestrator.Run(context.Background(), testAgentTrigger())
	if !errors.Is(err, writerErr) {
		t.Fatalf("got error %v, want writer error", err)
	}
	if audit.calls != 1 || audit.lastInput.Status != agentaudit.StatusSucceeded {
		t.Fatalf("audit should record successful runtime before writer failure: calls=%d input=%+v", audit.calls, audit.lastInput)
	}
	if writer.calls != 1 {
		t.Fatalf("writer calls = %d, want 1", writer.calls)
	}
}

func newTestAgentRunOrchestrator(t *testing.T, runtime agentruntime.Runtime, builder RuntimeRequestBuilder, audit AgentRunAuditRecorder, writer ResponseWriter) *AgentRunOrchestrator {
	t.Helper()
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime:        runtime,
		RequestBuilder: builder,
		Audit:          audit,
		Writer:         writer,
		Now: func() time.Time {
			return time.Unix(100, 0)
		},
	})
	if err != nil {
		t.Fatalf("new orchestrator: %v", err)
	}
	return orchestrator
}

func testAgentTrigger() AgentTrigger {
	return AgentTrigger{
		RequestID:          "evt_1:agent_1",
		EventID:            "evt_1",
		OperationID:        "op_1",
		TraceID:            "trace_1",
		TriggerType:        TriggerTypeUserPrivateMessage,
		AgentUserID:        "agent_1",
		RequestingUserID:   "user_1",
		ConversationID:     "single:agent_1:user_1",
		ConversationType:   ConversationTypeSingle,
		TriggerMessageID:   "msg_user_1",
		TriggerSeq:         7,
		PromptText:         "hello",
		ReplyToMessageID:   "msg_user_1",
		SourceMessageID:    "msg_user_1",
		SourceMessageSeq:   7,
		SourceMessageText:  "hello",
		SourceContentType:  ContentTypeText,
		TargetAgentUserIDs: []string{"agent_1"},
	}
}

type recordingAgentRuntime struct {
	calls   int
	lastReq agentruntime.RunRequest
	result  agentruntime.RunResult
	err     error
}

func (r *recordingAgentRuntime) Run(_ context.Context, req agentruntime.RunRequest) (agentruntime.RunResult, error) {
	r.calls++
	r.lastReq = req
	if r.err != nil {
		return agentruntime.RunResult{}, r.err
	}
	return r.result, nil
}

type recordingRuntimeRequestBuilder struct {
	calls       int
	lastTrigger AgentTrigger
	request     agentruntime.RunRequest
	err         error
}

func (b *recordingRuntimeRequestBuilder) BuildRuntimeRequest(_ context.Context, trigger AgentTrigger) (agentruntime.RunRequest, error) {
	b.calls++
	b.lastTrigger = trigger
	if b.err != nil {
		return agentruntime.RunRequest{}, b.err
	}
	return b.request, nil
}

type recordingAgentRunAuditRecorder struct {
	calls     int
	lastInput agentaudit.CreateRunInput
	run       agentaudit.AgentRun
	err       error
}

func (r *recordingAgentRunAuditRecorder) RecordAgentRun(_ context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	r.calls++
	r.lastInput = input
	if r.err != nil {
		return agentaudit.AgentRun{}, r.err
	}
	return r.run, nil
}

type recordingAgentResponseWriter struct {
	calls   int
	lastReq AgentResponseRequest
	result  AgentResponseResult
	err     error
}

func (w *recordingAgentResponseWriter) WriteAgentResponse(_ context.Context, req AgentResponseRequest) (AgentResponseResult, error) {
	w.calls++
	w.lastReq = req
	if w.err != nil {
		return AgentResponseResult{}, w.err
	}
	return w.result, nil
}

func validRuntimeRequestForTrigger(trigger AgentTrigger) agentruntime.RunRequest {
	promptText := trigger.PromptText
	if promptText == "" {
		promptText = trigger.SourceMessageText
	}
	return agentruntime.RunRequest{
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
		PromptText:         promptText,
		ReplyToMessageID:   trigger.ReplyToMessageID,
		SourceAgentRunID:   trigger.SourceAgentRunID,
		SourceAgentUserID:  trigger.SourceAgentUserID,
		SourceMessageID:    trigger.SourceMessageID,
		SourceMessageSeq:   trigger.SourceMessageSeq,
		SourceMessageText:  trigger.SourceMessageText,
		SourceContentType:  trigger.SourceContentType,
		TargetAgentUserIDs: append([]string(nil), trigger.TargetAgentUserIDs...),
		Agent: agentruntime.AgentConfig{
			AgentID:     "agent_profile_1",
			AgentUserID: trigger.AgentUserID,
			Name:        "Support Agent",
			Status:      agentruntime.AgentStatusActive,
			Prompt: agentruntime.PromptRef{
				PromptID: "prompt_1",
				Content:  "Answer using the configured support policy.",
				Version:  "v1",
			},
			Model: agentruntime.ModelConfig{
				Provider: "deepseek",
				Model:    "deepseek-v4-pro",
			},
			Policy: agentruntime.RuntimePolicy{
				MaxToolCalls:                   3,
				MaxRunDuration:                 time.Minute,
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
