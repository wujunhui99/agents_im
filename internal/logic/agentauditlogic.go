package logic

import (
	"context"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/common/share/agentaudit"
	"github.com/wujunhui99/agents_im/internal/repository"
)

type AgentAuditLogic struct {
	repo repository.AgentAuditRepository
}

type AgentRunAudit struct {
	Run         agentaudit.AgentRun
	ToolCalls   []agentaudit.AgentToolCall
	FileReads   []agentaudit.AgentFileRead
	PythonExecs []agentaudit.AgentPythonExec
}

func NewAgentAuditLogic(repo repository.AgentAuditRepository) *AgentAuditLogic {
	return &AgentAuditLogic{repo: repo}
}

func (l *AgentAuditLogic) RecordAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	if l == nil || l.repo == nil {
		return agentaudit.AgentRun{}, apperror.Internal("agent audit repository is not configured")
	}
	return l.repo.CreateAgentRun(ctx, input)
}

func (l *AgentAuditLogic) RecordAgentToolCall(ctx context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
	if l == nil || l.repo == nil {
		return agentaudit.AgentToolCall{}, apperror.Internal("agent audit repository is not configured")
	}
	return l.repo.CreateAgentToolCall(ctx, input)
}

func (l *AgentAuditLogic) RecordAgentFileRead(ctx context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
	if l == nil || l.repo == nil {
		return agentaudit.AgentFileRead{}, apperror.Internal("agent audit repository is not configured")
	}
	return l.repo.CreateAgentFileRead(ctx, input)
}

func (l *AgentAuditLogic) RecordAgentPythonExec(ctx context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
	if l == nil || l.repo == nil {
		return agentaudit.AgentPythonExec{}, apperror.Internal("agent audit repository is not configured")
	}
	return l.repo.CreateAgentPythonExec(ctx, input)
}

func (l *AgentAuditLogic) GetAgentRunAudit(ctx context.Context, runID string) (AgentRunAudit, error) {
	if l == nil || l.repo == nil {
		return AgentRunAudit{}, apperror.Internal("agent audit repository is not configured")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return AgentRunAudit{}, apperror.InvalidArgument("run_id is required")
	}

	run, err := l.repo.GetAgentRun(ctx, runID)
	if err != nil {
		return AgentRunAudit{}, err
	}
	toolCalls, err := l.repo.ListAgentToolCallsByRunID(ctx, runID)
	if err != nil {
		return AgentRunAudit{}, err
	}
	fileReads, err := l.repo.ListAgentFileReadsByRunID(ctx, runID)
	if err != nil {
		return AgentRunAudit{}, err
	}
	pythonExecs, err := l.repo.ListAgentPythonExecsByRunID(ctx, runID)
	if err != nil {
		return AgentRunAudit{}, err
	}
	return AgentRunAudit{
		Run:         run,
		ToolCalls:   toolCalls,
		FileReads:   fileReads,
		PythonExecs: pythonExecs,
	}, nil
}
