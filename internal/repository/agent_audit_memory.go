package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/wujunhui99/agents_im/common/share/agentaudit"
)

var _ AgentAuditRepository = (*MemoryAgentAuditRepository)(nil)

type MemoryAgentAuditRepository struct {
	mu          sync.RWMutex
	runs        map[string]agentaudit.AgentRun
	toolCalls   map[string]agentaudit.AgentToolCall
	fileReads   map[string]agentaudit.AgentFileRead
	pythonExecs map[string]agentaudit.AgentPythonExec
	now         func() time.Time
	newID       func(prefix string) (string, error)
}

func NewMemoryAgentAuditRepository() *MemoryAgentAuditRepository {
	return &MemoryAgentAuditRepository{
		runs:        make(map[string]agentaudit.AgentRun),
		toolCalls:   make(map[string]agentaudit.AgentToolCall),
		fileReads:   make(map[string]agentaudit.AgentFileRead),
		pythonExecs: make(map[string]agentaudit.AgentPythonExec),
		now:         time.Now,
		newID:       newAuditID,
	}
}

func (r *MemoryAgentAuditRepository) CreateAgentRun(_ context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	normalized, err := agentaudit.NormalizeCreateRunInput(input)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	if normalized.RunID == "" {
		normalized.RunID, err = r.newID("run")
		if err != nil {
			return agentaudit.AgentRun{}, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[normalized.RunID]; exists {
		return agentaudit.AgentRun{}, apperror.AlreadyExists("agent run audit already exists")
	}
	now := r.now().UTC()
	run := agentaudit.AgentRun{
		RunID:            normalized.RunID,
		AgentID:          normalized.AgentID,
		ConversationID:   normalized.ConversationID,
		TriggerMessageID: normalized.TriggerMessageID,
		RequestingUserID: normalized.RequestingUserID,
		Status:           normalized.Status,
		InputSummary:     normalized.InputSummary.Clone(),
		OutputSummary:    normalized.OutputSummary.Clone(),
		OutputMessageID:  normalized.OutputMessageID,
		ErrorCode:        normalized.ErrorCode,
		ErrorMessage:     normalized.ErrorMessage,
		TraceID:          normalized.TraceID,
		RequestID:        normalized.RequestID,
		StartedAt:        defaultAuditTime(normalized.StartedAt, now),
		FinishedAt:       normalized.FinishedAt,
		CreatedAt:        now,
	}
	r.runs[run.RunID] = run.Clone()
	return run.Clone(), nil
}

func (r *MemoryAgentAuditRepository) GetAgentRun(_ context.Context, runID string) (agentaudit.AgentRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("run_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, exists := r.runs[runID]
	if !exists {
		return agentaudit.AgentRun{}, apperror.NotFound("agent run audit not found")
	}
	return run.Clone(), nil
}

func (r *MemoryAgentAuditRepository) ListAgentRuns(_ context.Context, filter AgentRunFilter) ([]agentaudit.AgentRun, error) {
	limit := normalizeAdminLimit(filter.Limit, 20, 100)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(filter.Status)

	r.mu.RLock()
	defer r.mu.RUnlock()

	runs := make([]agentaudit.AgentRun, 0, len(r.runs))
	for _, run := range r.runs {
		if status != "" && string(run.Status) != status {
			continue
		}
		runs = append(runs, run.Clone())
	}
	sort.Slice(runs, func(i int, j int) bool {
		if runs[i].CreatedAt.Equal(runs[j].CreatedAt) {
			return runs[i].RunID < runs[j].RunID
		}
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})
	if offset >= len(runs) {
		return []agentaudit.AgentRun{}, nil
	}
	runs = runs[offset:]
	if len(runs) > limit {
		runs = runs[:limit]
	}
	return runs, nil
}

func (r *MemoryAgentAuditRepository) GetAgentRunByTraceID(_ context.Context, traceID string) (agentaudit.AgentRun, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("trace_id is required")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []agentaudit.AgentRun
	for _, run := range r.runs {
		if run.TraceID == traceID {
			matches = append(matches, run.Clone())
		}
	}
	if len(matches) == 0 {
		return agentaudit.AgentRun{}, apperror.NotFound("agent run audit not found")
	}
	sort.Slice(matches, func(i int, j int) bool {
		if matches[i].CreatedAt.Equal(matches[j].CreatedAt) {
			return matches[i].RunID < matches[j].RunID
		}
		return matches[i].CreatedAt.After(matches[j].CreatedAt)
	})
	return matches[0].Clone(), nil
}

func (r *MemoryAgentAuditRepository) CountAgentRuns(_ context.Context, status string) (int64, error) {
	status = strings.TrimSpace(status)
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int64
	for _, run := range r.runs {
		if status == "" || string(run.Status) == status {
			count++
		}
	}
	return count, nil
}

func (r *MemoryAgentAuditRepository) CreateAgentToolCall(_ context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
	normalized, err := agentaudit.NormalizeCreateToolCallInput(input)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	if normalized.ToolCallID == "" {
		normalized.ToolCallID, err = r.newID("tool_call")
		if err != nil {
			return agentaudit.AgentToolCall{}, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[normalized.RunID]; !exists {
		return agentaudit.AgentToolCall{}, apperror.NotFound("agent run audit not found")
	}
	if _, exists := r.toolCalls[normalized.ToolCallID]; exists {
		return agentaudit.AgentToolCall{}, apperror.AlreadyExists("agent tool call audit already exists")
	}
	now := r.now().UTC()
	call := agentaudit.AgentToolCall{
		ToolCallID:    normalized.ToolCallID,
		RunID:         normalized.RunID,
		AgentID:       normalized.AgentID,
		ToolID:        normalized.ToolID,
		ToolName:      normalized.ToolName,
		Status:        normalized.Status,
		InputSummary:  normalized.InputSummary.Clone(),
		OutputSummary: normalized.OutputSummary.Clone(),
		DurationMs:    normalized.DurationMs,
		ErrorCode:     normalized.ErrorCode,
		ErrorMessage:  normalized.ErrorMessage,
		TraceID:       normalized.TraceID,
		RequestID:     normalized.RequestID,
		StartedAt:     defaultAuditTime(normalized.StartedAt, now),
		FinishedAt:    normalized.FinishedAt,
		CreatedAt:     now,
	}
	r.toolCalls[call.ToolCallID] = call.Clone()
	return call.Clone(), nil
}

func (r *MemoryAgentAuditRepository) GetAgentToolCall(_ context.Context, toolCallID string) (agentaudit.AgentToolCall, error) {
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		return agentaudit.AgentToolCall{}, apperror.InvalidArgument("tool_call_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	call, exists := r.toolCalls[toolCallID]
	if !exists {
		return agentaudit.AgentToolCall{}, apperror.NotFound("agent tool call audit not found")
	}
	return call.Clone(), nil
}

func (r *MemoryAgentAuditRepository) ListAgentToolCallsByRunID(_ context.Context, runID string) ([]agentaudit.AgentToolCall, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, apperror.InvalidArgument("run_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, exists := r.runs[runID]; !exists {
		return nil, apperror.NotFound("agent run audit not found")
	}
	calls := make([]agentaudit.AgentToolCall, 0)
	for _, call := range r.toolCalls {
		if call.RunID == runID {
			calls = append(calls, call.Clone())
		}
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].CreatedAt.Equal(calls[j].CreatedAt) {
			return calls[i].ToolCallID < calls[j].ToolCallID
		}
		return calls[i].CreatedAt.Before(calls[j].CreatedAt)
	})
	return calls, nil
}

func (r *MemoryAgentAuditRepository) CreateAgentFileRead(_ context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
	normalized, err := agentaudit.NormalizeCreateFileReadInput(input)
	if err != nil {
		return agentaudit.AgentFileRead{}, err
	}
	if normalized.FileReadID == "" {
		normalized.FileReadID, err = r.newID("file_read")
		if err != nil {
			return agentaudit.AgentFileRead{}, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[normalized.RunID]; !exists {
		return agentaudit.AgentFileRead{}, apperror.NotFound("agent run audit not found")
	}
	if _, exists := r.fileReads[normalized.FileReadID]; exists {
		return agentaudit.AgentFileRead{}, apperror.AlreadyExists("agent file read audit already exists")
	}
	now := r.now().UTC()
	read := agentaudit.AgentFileRead{
		FileReadID:     normalized.FileReadID,
		RunID:          normalized.RunID,
		AgentID:        normalized.AgentID,
		SkillID:        normalized.SkillID,
		FileID:         normalized.FileID,
		ObjectKey:      normalized.ObjectKey,
		SHA256:         normalized.SHA256,
		Status:         normalized.Status,
		ByteCount:      normalized.ByteCount,
		ContentSummary: normalized.ContentSummary.Clone(),
		ErrorCode:      normalized.ErrorCode,
		ErrorMessage:   normalized.ErrorMessage,
		TraceID:        normalized.TraceID,
		RequestID:      normalized.RequestID,
		StartedAt:      defaultAuditTime(normalized.StartedAt, now),
		FinishedAt:     normalized.FinishedAt,
		CreatedAt:      now,
	}
	r.fileReads[read.FileReadID] = read.Clone()
	return read.Clone(), nil
}

func (r *MemoryAgentAuditRepository) GetAgentFileRead(_ context.Context, fileReadID string) (agentaudit.AgentFileRead, error) {
	fileReadID = strings.TrimSpace(fileReadID)
	if fileReadID == "" {
		return agentaudit.AgentFileRead{}, apperror.InvalidArgument("file_read_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	read, exists := r.fileReads[fileReadID]
	if !exists {
		return agentaudit.AgentFileRead{}, apperror.NotFound("agent file read audit not found")
	}
	return read.Clone(), nil
}

func (r *MemoryAgentAuditRepository) ListAgentFileReadsByRunID(_ context.Context, runID string) ([]agentaudit.AgentFileRead, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, apperror.InvalidArgument("run_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, exists := r.runs[runID]; !exists {
		return nil, apperror.NotFound("agent run audit not found")
	}
	reads := make([]agentaudit.AgentFileRead, 0)
	for _, read := range r.fileReads {
		if read.RunID == runID {
			reads = append(reads, read.Clone())
		}
	}
	sort.Slice(reads, func(i, j int) bool {
		if reads[i].CreatedAt.Equal(reads[j].CreatedAt) {
			return reads[i].FileReadID < reads[j].FileReadID
		}
		return reads[i].CreatedAt.Before(reads[j].CreatedAt)
	})
	return reads, nil
}

func (r *MemoryAgentAuditRepository) CreateAgentPythonExec(_ context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
	normalized, err := agentaudit.NormalizeCreatePythonExecInput(input)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	if normalized.PythonExecID == "" {
		normalized.PythonExecID, err = r.newID("python_exec")
		if err != nil {
			return agentaudit.AgentPythonExec{}, err
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[normalized.RunID]; !exists {
		return agentaudit.AgentPythonExec{}, apperror.NotFound("agent run audit not found")
	}
	if _, exists := r.pythonExecs[normalized.PythonExecID]; exists {
		return agentaudit.AgentPythonExec{}, apperror.AlreadyExists("agent python exec audit already exists")
	}
	now := r.now().UTC()
	exec := agentaudit.AgentPythonExec{
		PythonExecID:     normalized.PythonExecID,
		RunID:            normalized.RunID,
		AgentID:          normalized.AgentID,
		SandboxRequestID: normalized.SandboxRequestID,
		Status:           normalized.Status,
		CodeSummary:      normalized.CodeSummary.Clone(),
		ResourceSummary:  normalized.ResourceSummary.Clone(),
		StdoutSummary:    normalized.StdoutSummary.Clone(),
		StderrSummary:    normalized.StderrSummary.Clone(),
		ResultSummary:    normalized.ResultSummary.Clone(),
		ErrorCode:        normalized.ErrorCode,
		ErrorMessage:     normalized.ErrorMessage,
		TraceID:          normalized.TraceID,
		RequestID:        normalized.RequestID,
		StartedAt:        defaultAuditTime(normalized.StartedAt, now),
		FinishedAt:       normalized.FinishedAt,
		CreatedAt:        now,
	}
	r.pythonExecs[exec.PythonExecID] = exec.Clone()
	return exec.Clone(), nil
}

func (r *MemoryAgentAuditRepository) GetAgentPythonExec(_ context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error) {
	pythonExecID = strings.TrimSpace(pythonExecID)
	if pythonExecID == "" {
		return agentaudit.AgentPythonExec{}, apperror.InvalidArgument("python_exec_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	exec, exists := r.pythonExecs[pythonExecID]
	if !exists {
		return agentaudit.AgentPythonExec{}, apperror.NotFound("agent python exec audit not found")
	}
	return exec.Clone(), nil
}

func (r *MemoryAgentAuditRepository) ListAgentPythonExecsByRunID(_ context.Context, runID string) ([]agentaudit.AgentPythonExec, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, apperror.InvalidArgument("run_id is required")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, exists := r.runs[runID]; !exists {
		return nil, apperror.NotFound("agent run audit not found")
	}
	execs := make([]agentaudit.AgentPythonExec, 0)
	for _, exec := range r.pythonExecs {
		if exec.RunID == runID {
			execs = append(execs, exec.Clone())
		}
	}
	sort.Slice(execs, func(i, j int) bool {
		if execs[i].CreatedAt.Equal(execs[j].CreatedAt) {
			return execs[i].PythonExecID < execs[j].PythonExecID
		}
		return execs[i].CreatedAt.Before(execs[j].CreatedAt)
	})
	return execs, nil
}

func defaultAuditTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.UTC()
}

func newAuditID(prefix string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return strings.TrimSpace(prefix) + "_" + hex.EncodeToString(raw[:]), nil
}
