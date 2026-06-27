package agaudit

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/service/agent/rpc/internal/model"
)

// ModelStore 是 Store 的 goctl model 实现（背靠 agent 审计四表）。写路径直接委托各表 model 的
// custom insert 方法；只读的 List*ByRunID 先确认 run 存在（保留旧 internal/repository 的
// run-not-found 语义）再列子表。
type ModelStore struct {
	runs        model.AgentRunsModel
	toolCalls   model.AgentToolCallsModel
	fileReads   model.AgentFileReadsModel
	pythonExecs model.AgentPythonExecsModel
}

var _ Store = (*ModelStore)(nil)

// NewModelStore 用数据源构建 model-backed Store。
func NewModelStore(dataSource string) *ModelStore {
	return NewModelStoreFromConn(postgres.New(dataSource))
}

// NewModelStoreFromConn 用已建连接构建 model-backed Store。
func NewModelStoreFromConn(conn sqlx.SqlConn) *ModelStore {
	return &ModelStore{
		runs:        model.NewAgentRunsModel(conn),
		toolCalls:   model.NewAgentToolCallsModel(conn),
		fileReads:   model.NewAgentFileReadsModel(conn),
		pythonExecs: model.NewAgentPythonExecsModel(conn),
	}
}

func (s *ModelStore) CreateAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	return s.runs.InsertRunAudit(ctx, input)
}

func (s *ModelStore) GetAgentRun(ctx context.Context, runID string) (agentaudit.AgentRun, error) {
	return s.runs.FindRunAudit(ctx, runID)
}

func (s *ModelStore) ListAgentRuns(ctx context.Context, filter RunFilter) ([]agentaudit.AgentRun, error) {
	return s.runs.ListRunAudits(ctx, filter.Status, filter.Limit, filter.Offset)
}

func (s *ModelStore) GetAgentRunByTraceID(ctx context.Context, traceID string) (agentaudit.AgentRun, error) {
	return s.runs.FindRunAuditByTraceID(ctx, traceID)
}

func (s *ModelStore) CountAgentRuns(ctx context.Context, status string) (int64, error) {
	return s.runs.CountRunAudits(ctx, status)
}

func (s *ModelStore) CreateAgentToolCall(ctx context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
	return s.toolCalls.InsertToolCallAudit(ctx, input)
}

func (s *ModelStore) GetAgentToolCall(ctx context.Context, toolCallID string) (agentaudit.AgentToolCall, error) {
	return s.toolCalls.FindToolCallAudit(ctx, toolCallID)
}

func (s *ModelStore) ListAgentToolCallsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentToolCall, error) {
	if _, err := s.runs.FindRunAudit(ctx, runID); err != nil {
		return nil, err
	}
	return s.toolCalls.ListToolCallAuditsByRunID(ctx, runID)
}

func (s *ModelStore) CreateAgentFileRead(ctx context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
	return s.fileReads.InsertFileReadAudit(ctx, input)
}

func (s *ModelStore) GetAgentFileRead(ctx context.Context, fileReadID string) (agentaudit.AgentFileRead, error) {
	return s.fileReads.FindFileReadAudit(ctx, fileReadID)
}

func (s *ModelStore) ListAgentFileReadsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentFileRead, error) {
	if _, err := s.runs.FindRunAudit(ctx, runID); err != nil {
		return nil, err
	}
	return s.fileReads.ListFileReadAuditsByRunID(ctx, runID)
}

func (s *ModelStore) CreateAgentPythonExec(ctx context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
	return s.pythonExecs.InsertPythonExecAudit(ctx, input)
}

func (s *ModelStore) GetAgentPythonExec(ctx context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error) {
	return s.pythonExecs.FindPythonExecAudit(ctx, pythonExecID)
}

func (s *ModelStore) ListAgentPythonExecsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentPythonExec, error) {
	if _, err := s.runs.FindRunAudit(ctx, runID); err != nil {
		return nil, err
	}
	return s.pythonExecs.ListPythonExecAuditsByRunID(ctx, runID)
}
