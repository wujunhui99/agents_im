// Package agaudit owns the agent-run audit data layer for agent-rpc: the
// append-only agent_runs / agent_tool_calls / agent_file_reads / agent_python_execs
// tables (migration 002/013). These are agent-domain self-owned audit data (no
// cross-domain reads), so this replaces the keystone
// internal/repository.AgentAuditRepository + internal/logic.AgentAuditLogic
// outright (issue #616, split from #344/#394) — agent-rpc orchestrator/consumer
// no longer imports internal/ agent_audit, and admin-rpc reads traces via the
// agent-rpc gRPC face instead of internal/repository.
//
// Store is the data interface (goctl model backed in prod via ModelStore,
// in-memory in tests/demo via MemoryStore). The orchestrator's audit recorder and
// the agent-rpc audit read gRPC handlers depend on this interface.
package agaudit

import (
	"context"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
)

// RunFilter 是 ListAgentRuns 的过滤/分页入参（admin traces/dashboard 用）。
type RunFilter struct {
	Status string
	Limit  int
	Offset int
}

// Store 是 agent 审计四表的数据访问接口：写路径（orchestrator 记录 run/tool_call/file_read/
// python_exec）+ 只读路径（admin-rpc 经 agent-rpc gRPC 读 traces/dashboard）。prod 注入
// ModelStore（goctl），单测注入 MemoryStore。
type Store interface {
	CreateAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error)
	GetAgentRun(ctx context.Context, runID string) (agentaudit.AgentRun, error)
	ListAgentRuns(ctx context.Context, filter RunFilter) ([]agentaudit.AgentRun, error)
	GetAgentRunByTraceID(ctx context.Context, traceID string) (agentaudit.AgentRun, error)
	CountAgentRuns(ctx context.Context, status string) (int64, error)

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
