package agaudit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
)

// MemoryStore 是 Store 的内存实现（单测 / demo fixture）。append-only：行写后不可改，重复主键
// 返回 AlreadyExists；子表写入要求 run 已存在（FK 语义）。
type MemoryStore struct {
	mu          sync.RWMutex
	runs        map[string]agentaudit.AgentRun
	toolCalls   map[string]agentaudit.AgentToolCall
	fileReads   map[string]agentaudit.AgentFileRead
	pythonExecs map[string]agentaudit.AgentPythonExec
	now         func() time.Time
	newID       func() (string, error)
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore 构建空的内存审计 Store。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs:        make(map[string]agentaudit.AgentRun),
		toolCalls:   make(map[string]agentaudit.AgentToolCall),
		fileReads:   make(map[string]agentaudit.AgentFileRead),
		pythonExecs: make(map[string]agentaudit.AgentPythonExec),
		now:         time.Now,
		newID:       newMemoryAuditID,
	}
}

func (s *MemoryStore) CreateAgentRun(_ context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	normalized, err := agentaudit.NormalizeCreateRunInput(input)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	if normalized.RunID == "" {
		normalized.RunID, err = s.newID()
		if err != nil {
			return agentaudit.AgentRun{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[normalized.RunID]; exists {
		return agentaudit.AgentRun{}, apperror.AlreadyExists("agent run audit already exists")
	}
	now := s.now().UTC()
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
		StartedAt:        memoryAuditTime(normalized.StartedAt, now),
		FinishedAt:       normalized.FinishedAt,
		CreatedAt:        now,
	}
	s.runs[run.RunID] = run.Clone()
	return run.Clone(), nil
}

func (s *MemoryStore) GetAgentRun(_ context.Context, runID string) (agentaudit.AgentRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("run_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, exists := s.runs[runID]
	if !exists {
		return agentaudit.AgentRun{}, apperror.NotFound("agent run audit not found")
	}
	return run.Clone(), nil
}

func (s *MemoryStore) ListAgentRuns(_ context.Context, filter RunFilter) ([]agentaudit.AgentRun, error) {
	limit := normalizeAgentAuditLimit(filter.Limit, 20, 100)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(filter.Status)

	s.mu.RLock()
	defer s.mu.RUnlock()

	runs := make([]agentaudit.AgentRun, 0, len(s.runs))
	for _, run := range s.runs {
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

func (s *MemoryStore) GetAgentRunByTraceID(_ context.Context, traceID string) (agentaudit.AgentRun, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("trace_id is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var matches []agentaudit.AgentRun
	for _, run := range s.runs {
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

func (s *MemoryStore) CountAgentRuns(_ context.Context, status string) (int64, error) {
	status = strings.TrimSpace(status)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, run := range s.runs {
		if status == "" || string(run.Status) == status {
			count++
		}
	}
	return count, nil
}

func (s *MemoryStore) CreateAgentToolCall(_ context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
	normalized, err := agentaudit.NormalizeCreateToolCallInput(input)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	if normalized.ToolCallID == "" {
		normalized.ToolCallID, err = s.newID()
		if err != nil {
			return agentaudit.AgentToolCall{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[normalized.RunID]; !exists {
		return agentaudit.AgentToolCall{}, apperror.NotFound("agent run audit not found")
	}
	if _, exists := s.toolCalls[normalized.ToolCallID]; exists {
		return agentaudit.AgentToolCall{}, apperror.AlreadyExists("agent tool call audit already exists")
	}
	now := s.now().UTC()
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
		StartedAt:     memoryAuditTime(normalized.StartedAt, now),
		FinishedAt:    normalized.FinishedAt,
		CreatedAt:     now,
	}
	s.toolCalls[call.ToolCallID] = call.Clone()
	return call.Clone(), nil
}

func (s *MemoryStore) GetAgentToolCall(_ context.Context, toolCallID string) (agentaudit.AgentToolCall, error) {
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		return agentaudit.AgentToolCall{}, apperror.InvalidArgument("tool_call_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	call, exists := s.toolCalls[toolCallID]
	if !exists {
		return agentaudit.AgentToolCall{}, apperror.NotFound("agent tool call audit not found")
	}
	return call.Clone(), nil
}

func (s *MemoryStore) ListAgentToolCallsByRunID(_ context.Context, runID string) ([]agentaudit.AgentToolCall, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, apperror.InvalidArgument("run_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.runs[runID]; !exists {
		return nil, apperror.NotFound("agent run audit not found")
	}
	calls := make([]agentaudit.AgentToolCall, 0)
	for _, call := range s.toolCalls {
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

func (s *MemoryStore) CreateAgentFileRead(_ context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
	normalized, err := agentaudit.NormalizeCreateFileReadInput(input)
	if err != nil {
		return agentaudit.AgentFileRead{}, err
	}
	if normalized.FileReadID == "" {
		normalized.FileReadID, err = s.newID()
		if err != nil {
			return agentaudit.AgentFileRead{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[normalized.RunID]; !exists {
		return agentaudit.AgentFileRead{}, apperror.NotFound("agent run audit not found")
	}
	if _, exists := s.fileReads[normalized.FileReadID]; exists {
		return agentaudit.AgentFileRead{}, apperror.AlreadyExists("agent file read audit already exists")
	}
	now := s.now().UTC()
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
		StartedAt:      memoryAuditTime(normalized.StartedAt, now),
		FinishedAt:     normalized.FinishedAt,
		CreatedAt:      now,
	}
	s.fileReads[read.FileReadID] = read.Clone()
	return read.Clone(), nil
}

func (s *MemoryStore) GetAgentFileRead(_ context.Context, fileReadID string) (agentaudit.AgentFileRead, error) {
	fileReadID = strings.TrimSpace(fileReadID)
	if fileReadID == "" {
		return agentaudit.AgentFileRead{}, apperror.InvalidArgument("file_read_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	read, exists := s.fileReads[fileReadID]
	if !exists {
		return agentaudit.AgentFileRead{}, apperror.NotFound("agent file read audit not found")
	}
	return read.Clone(), nil
}

func (s *MemoryStore) ListAgentFileReadsByRunID(_ context.Context, runID string) ([]agentaudit.AgentFileRead, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, apperror.InvalidArgument("run_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.runs[runID]; !exists {
		return nil, apperror.NotFound("agent run audit not found")
	}
	reads := make([]agentaudit.AgentFileRead, 0)
	for _, read := range s.fileReads {
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

func (s *MemoryStore) CreateAgentPythonExec(_ context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
	normalized, err := agentaudit.NormalizeCreatePythonExecInput(input)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	if normalized.PythonExecID == "" {
		normalized.PythonExecID, err = s.newID()
		if err != nil {
			return agentaudit.AgentPythonExec{}, err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[normalized.RunID]; !exists {
		return agentaudit.AgentPythonExec{}, apperror.NotFound("agent run audit not found")
	}
	if _, exists := s.pythonExecs[normalized.PythonExecID]; exists {
		return agentaudit.AgentPythonExec{}, apperror.AlreadyExists("agent python exec audit already exists")
	}
	now := s.now().UTC()
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
		StartedAt:        memoryAuditTime(normalized.StartedAt, now),
		FinishedAt:       normalized.FinishedAt,
		CreatedAt:        now,
	}
	s.pythonExecs[exec.PythonExecID] = exec.Clone()
	return exec.Clone(), nil
}

func (s *MemoryStore) GetAgentPythonExec(_ context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error) {
	pythonExecID = strings.TrimSpace(pythonExecID)
	if pythonExecID == "" {
		return agentaudit.AgentPythonExec{}, apperror.InvalidArgument("python_exec_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	exec, exists := s.pythonExecs[pythonExecID]
	if !exists {
		return agentaudit.AgentPythonExec{}, apperror.NotFound("agent python exec audit not found")
	}
	return exec.Clone(), nil
}

func (s *MemoryStore) ListAgentPythonExecsByRunID(_ context.Context, runID string) ([]agentaudit.AgentPythonExec, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, apperror.InvalidArgument("run_id is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, exists := s.runs[runID]; !exists {
		return nil, apperror.NotFound("agent run audit not found")
	}
	execs := make([]agentaudit.AgentPythonExec, 0)
	for _, exec := range s.pythonExecs {
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

func normalizeAgentAuditLimit(value int, fallback int, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func memoryAuditTime(value time.Time, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.UTC()
}

func newMemoryAuditID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
