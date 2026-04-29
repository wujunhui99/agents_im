package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/wujunhui99/agents_im/internal/repository"
)

func TestAgentAuditLogicReturnsAuditWriteErrors(t *testing.T) {
	wantErr := errors.New("audit store unavailable")
	logic := NewAgentAuditLogic(failingAgentAuditRepository{err: wantErr})

	_, err := logic.RecordAgentRun(context.Background(), agentaudit.CreateRunInput{
		AgentID: "agent_1",
		Status:  agentaudit.StatusStarted,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RecordAgentRun error = %v, want %v", err, wantErr)
	}
}

func TestAgentAuditLogicGetsRunAuditByRunID(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryAgentAuditRepository()
	logic := NewAgentAuditLogic(repo)

	run, err := logic.RecordAgentRun(ctx, agentaudit.CreateRunInput{
		RunID:   "run_logic_1",
		AgentID: "agent_1",
		Status:  agentaudit.StatusSucceeded,
	})
	if err != nil {
		t.Fatalf("record run: %v", err)
	}
	if _, err := logic.RecordAgentToolCall(ctx, agentaudit.CreateToolCallInput{
		ToolCallID: "tool_logic_1",
		RunID:      run.RunID,
		AgentID:    run.AgentID,
		ToolName:   "im.get_conversation_context",
		Status:     agentaudit.StatusSucceeded,
	}); err != nil {
		t.Fatalf("record tool call: %v", err)
	}

	audit, err := logic.GetAgentRunAudit(ctx, run.RunID)
	if err != nil {
		t.Fatalf("get run audit: %v", err)
	}
	if audit.Run.RunID != run.RunID || len(audit.ToolCalls) != 1 {
		t.Fatalf("run audit mismatch: %+v", audit)
	}
}

type failingAgentAuditRepository struct {
	err error
}

func (r failingAgentAuditRepository) CreateAgentRun(context.Context, agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	return agentaudit.AgentRun{}, r.err
}

func (r failingAgentAuditRepository) GetAgentRun(context.Context, string) (agentaudit.AgentRun, error) {
	return agentaudit.AgentRun{}, r.err
}

func (r failingAgentAuditRepository) CreateAgentToolCall(context.Context, agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
	return agentaudit.AgentToolCall{}, r.err
}

func (r failingAgentAuditRepository) GetAgentToolCall(context.Context, string) (agentaudit.AgentToolCall, error) {
	return agentaudit.AgentToolCall{}, r.err
}

func (r failingAgentAuditRepository) ListAgentToolCallsByRunID(context.Context, string) ([]agentaudit.AgentToolCall, error) {
	return nil, r.err
}

func (r failingAgentAuditRepository) CreateAgentFileRead(context.Context, agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
	return agentaudit.AgentFileRead{}, r.err
}

func (r failingAgentAuditRepository) GetAgentFileRead(context.Context, string) (agentaudit.AgentFileRead, error) {
	return agentaudit.AgentFileRead{}, r.err
}

func (r failingAgentAuditRepository) ListAgentFileReadsByRunID(context.Context, string) ([]agentaudit.AgentFileRead, error) {
	return nil, r.err
}

func (r failingAgentAuditRepository) CreateAgentPythonExec(context.Context, agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
	return agentaudit.AgentPythonExec{}, r.err
}

func (r failingAgentAuditRepository) GetAgentPythonExec(context.Context, string) (agentaudit.AgentPythonExec, error) {
	return agentaudit.AgentPythonExec{}, r.err
}

func (r failingAgentAuditRepository) ListAgentPythonExecsByRunID(context.Context, string) ([]agentaudit.AgentPythonExec, error) {
	return nil, r.err
}
