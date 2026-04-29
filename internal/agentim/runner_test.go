package agentim

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/logic"
)

func TestAgentRunOrchestratorSuccessWritesAuditedResponse(t *testing.T) {
	runtime := &recordingAgentRuntime{
		result: AgentRuntimeResult{
			FinalText:             " answer ",
			OutputSummary:         agentaudit.Summary{"model": "test-model"},
			AllowRecursiveTrigger: true,
		},
	}
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
	orchestrator := newTestAgentRunOrchestrator(t, runtime, audit, writer)

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
	if runtime.lastTrigger.RequestID != "evt_1:agent_1" {
		t.Fatalf("runtime did not receive normalized trigger: %+v", runtime.lastTrigger)
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
	if audit.lastInput.OutputSummary["final_text"] != "answer" || audit.lastInput.OutputSummary["model"] != "test-model" {
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

func TestAgentRunOrchestratorRuntimeFailureRecordsFailedAudit(t *testing.T) {
	runtimeErr := errors.New("runtime unavailable")
	runtime := &recordingAgentRuntime{err: runtimeErr}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_failed_1"},
	}
	writer := &recordingAgentResponseWriter{}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, audit, writer)

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
	if writer.calls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.calls)
	}
}

func TestAgentRunOrchestratorAuditFailureStopsResponseWrite(t *testing.T) {
	auditErr := errors.New("audit database unavailable")
	runtime := &recordingAgentRuntime{result: AgentRuntimeResult{FinalText: "answer"}}
	audit := &recordingAgentRunAuditRecorder{err: auditErr}
	writer := &recordingAgentResponseWriter{}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, audit, writer)

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
	runtime := &recordingAgentRuntime{result: AgentRuntimeResult{FinalText: " \t\n "}}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_empty_1"},
	}
	writer := &recordingAgentResponseWriter{}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, audit, writer)

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
	if writer.calls != 0 {
		t.Fatalf("writer calls = %d, want 0", writer.calls)
	}
}

func TestAgentRunOrchestratorPropagatesResponseWriterFailure(t *testing.T) {
	writerErr := errors.New("message service rejected response")
	runtime := &recordingAgentRuntime{result: AgentRuntimeResult{FinalText: "answer"}}
	audit := &recordingAgentRunAuditRecorder{
		run: agentaudit.AgentRun{RunID: "run_1"},
	}
	writer := &recordingAgentResponseWriter{err: writerErr}
	orchestrator := newTestAgentRunOrchestrator(t, runtime, audit, writer)

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

func newTestAgentRunOrchestrator(t *testing.T, runtime AgentRuntime, audit AgentRunAuditRecorder, writer ResponseWriter) *AgentRunOrchestrator {
	t.Helper()
	orchestrator, err := NewAgentRunOrchestrator(AgentRunOrchestratorConfig{
		Runtime: runtime,
		Audit:   audit,
		Writer:  writer,
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
	calls       int
	lastTrigger AgentTrigger
	result      AgentRuntimeResult
	err         error
}

func (r *recordingAgentRuntime) Run(_ context.Context, trigger AgentTrigger) (AgentRuntimeResult, error) {
	r.calls++
	r.lastTrigger = trigger
	if r.err != nil {
		return AgentRuntimeResult{}, r.err
	}
	return r.result, nil
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
