package repository

import (
	"context"

	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
)

type AgentAuditRepository interface {
	CreateAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error)
	GetAgentRun(ctx context.Context, runID string) (agentaudit.AgentRun, error)

	CreateAgentToolCall(ctx context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error)
	GetAgentToolCall(ctx context.Context, toolCallID string) (agentaudit.AgentToolCall, error)
	ListAgentToolCallsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentToolCall, error)

	CreateAgentFileRead(ctx context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error)
	GetAgentFileRead(ctx context.Context, fileReadID string) (agentaudit.AgentFileRead, error)
	ListAgentFileReadsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentFileRead, error)

	CreateAgentPythonExec(ctx context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error)
	GetAgentPythonExec(ctx context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error)
	ListAgentPythonExecsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentPythonExec, error)
}
