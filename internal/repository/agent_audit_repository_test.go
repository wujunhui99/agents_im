package repository

import (
	"context"
	"testing"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
)

func TestMemoryAgentAuditRepositoryAppendOnlyCreateListGetByRunID(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryAgentAuditRepository()
	startedAt := time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Second)

	run, err := repo.CreateAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:            "run_audit_1",
		AgentID:          "agent_1",
		ConversationID:   "single:usr_1:agent_1",
		TriggerMessageID: "msg_1",
		RequestingUserID: "usr_1",
		Status:           agentaudit.StatusSucceeded,
		InputSummary: agentaudit.Summary{
			"message":      "hello",
			"access_token": "must-not-leak",
		},
		OutputMessageID: "msg_agent_1",
		TraceID:         "trace_1",
		RequestID:       "req_1",
		StartedAt:       startedAt,
		FinishedAt:      finishedAt,
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.RunID != "run_audit_1" || run.CreatedAt.IsZero() {
		t.Fatalf("run metadata mismatch: %+v", run)
	}
	if run.InputSummary["access_token"] != agentaudit.RedactedValue {
		t.Fatalf("run input token was not redacted: %+v", run.InputSummary)
	}

	if _, err := repo.CreateAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:   run.RunID,
		AgentID: "agent_1",
		Status:  agentaudit.StatusStarted,
	}); err == nil {
		t.Fatal("duplicate run id should fail")
	} else if appErr := apperror.From(err); appErr.Code != apperror.CodeAlreadyExists {
		t.Fatalf("duplicate run id should be already exists, got %v", err)
	}

	toolCall, err := repo.CreateAgentToolCall(ctx, agentaudit.CreateToolCallInput{
		ToolCallID: "tool_call_1",
		RunID:      run.RunID,
		AgentID:    run.AgentID,
		ToolID:     "tool_builtin_context",
		ToolName:   "im.get_conversation_context",
		Status:     agentaudit.StatusSucceeded,
		InputSummary: agentaudit.Summary{
			"conversation_id": run.ConversationID,
			"api_token":       "must-not-leak",
		},
		OutputSummary: agentaudit.Summary{"message_count": 3},
		TraceID:       run.TraceID,
		RequestID:     run.RequestID,
		StartedAt:     startedAt,
		FinishedAt:    finishedAt,
	})
	if err != nil {
		t.Fatalf("create tool call: %v", err)
	}
	if toolCall.InputSummary["api_token"] != agentaudit.RedactedValue {
		t.Fatalf("tool call token was not redacted: %+v", toolCall.InputSummary)
	}

	if _, err := repo.CreateAgentFileRead(ctx, agentaudit.CreateFileReadInput{
		FileReadID:     "file_read_1",
		RunID:          run.RunID,
		AgentID:        run.AgentID,
		SkillID:        "skill_1",
		FileID:         "file_1",
		ObjectKey:      "skills/skill_1/versions/v1/SKILL.md",
		SHA256:         "abc123",
		Status:         agentaudit.StatusSucceeded,
		ByteCount:      128,
		ContentSummary: agentaudit.Summary{"line_count": 12},
		TraceID:        run.TraceID,
		RequestID:      run.RequestID,
		StartedAt:      startedAt,
		FinishedAt:     finishedAt,
	}); err != nil {
		t.Fatalf("create file read: %v", err)
	}

	pythonExec, err := repo.CreateAgentPythonExec(ctx, agentaudit.CreatePythonExecInput{
		PythonExecID:     "python_exec_1",
		RunID:            run.RunID,
		AgentID:          run.AgentID,
		SandboxRequestID: "sandbox_req_1",
		Status:           agentaudit.StatusFailed,
		Code:             "API_TOKEN = \"must-not-leak\"\nprint(API_TOKEN)\n",
		ResourceSummary:  agentaudit.Summary{"timeout_seconds": 10},
		ErrorCode:        "PYTHON_EXEC_FAILED",
		ErrorMessage:     "sandbox execution failed",
		TraceID:          run.TraceID,
		RequestID:        run.RequestID,
		StartedAt:        startedAt,
		FinishedAt:       finishedAt,
	})
	if err != nil {
		t.Fatalf("create python exec: %v", err)
	}
	if pythonExec.CodeSummary["sha256"] == "" || pythonExec.CodeSummary["size_bytes"] == nil {
		t.Fatalf("python code summary missing hash or size: %+v", pythonExec.CodeSummary)
	}
	if pythonExec.CodeSummary.String() == pythonExec.ErrorMessage {
		t.Fatalf("code summary should not be raw error text")
	}

	loadedRun, err := repo.GetAgentRun(ctx, run.RunID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if loadedRun.RunID != run.RunID || loadedRun.TraceID != "trace_1" || loadedRun.RequestID != "req_1" {
		t.Fatalf("loaded run mismatch: %+v", loadedRun)
	}

	toolCalls, err := repo.ListAgentToolCallsByRunID(ctx, run.RunID)
	if err != nil {
		t.Fatalf("list tool calls: %v", err)
	}
	if len(toolCalls) != 1 || toolCalls[0].ToolCallID != toolCall.ToolCallID {
		t.Fatalf("tool calls by run mismatch: %+v", toolCalls)
	}
	loadedToolCall, err := repo.GetAgentToolCall(ctx, toolCall.ToolCallID)
	if err != nil {
		t.Fatalf("get tool call: %v", err)
	}
	if loadedToolCall.RunID != run.RunID {
		t.Fatalf("loaded tool call run mismatch: %+v", loadedToolCall)
	}

	fileReads, err := repo.ListAgentFileReadsByRunID(ctx, run.RunID)
	if err != nil {
		t.Fatalf("list file reads: %v", err)
	}
	if len(fileReads) != 1 || fileReads[0].ObjectKey != "skills/skill_1/versions/v1/SKILL.md" {
		t.Fatalf("file reads by run mismatch: %+v", fileReads)
	}

	pythonExecs, err := repo.ListAgentPythonExecsByRunID(ctx, run.RunID)
	if err != nil {
		t.Fatalf("list python execs: %v", err)
	}
	if len(pythonExecs) != 1 || pythonExecs[0].PythonExecID != pythonExec.PythonExecID {
		t.Fatalf("python execs by run mismatch: %+v", pythonExecs)
	}
}

func TestMemoryAgentAuditRepositoryRequiresExistingRunForChildren(t *testing.T) {
	repo := NewMemoryAgentAuditRepository()
	ctx := context.Background()

	if _, err := repo.CreateAgentToolCall(ctx, agentaudit.CreateToolCallInput{
		RunID:    "missing_run",
		AgentID:  "agent_1",
		ToolName: "skill.read_file",
		Status:   agentaudit.StatusStarted,
	}); err == nil {
		t.Fatal("tool call with missing run should fail")
	} else if appErr := apperror.From(err); appErr.Code != apperror.CodeNotFound {
		t.Fatalf("missing run should be not found, got %v", err)
	}
}
