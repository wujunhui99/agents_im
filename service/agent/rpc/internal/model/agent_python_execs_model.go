package model

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/pkg/agentaudit"
	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentPythonExecsModel = (*customAgentPythonExecsModel)(nil)

type (
	// AgentPythonExecsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentPythonExecsModel.
	//
	// agent_python_execs 是 append-only 审计表；只暴露 insert + 只读 custom 方法。bigint keystone：
	// python_exec_id/run_id/agent_id 库内 bigint，写 `$n::bigint`、读 `col::text`。
	AgentPythonExecsModel interface {
		agentPythonExecsModel
		withSession(session sqlx.Session) AgentPythonExecsModel

		// InsertPythonExecAudit 写一条 python exec 审计行（python_exec_id 空则生成）。
		InsertPythonExecAudit(ctx context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error)
		// FindPythonExecAudit 按 python_exec_id 读；不存在返回 apperror.NotFound。
		FindPythonExecAudit(ctx context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error)
		// ListPythonExecAuditsByRunID 按 run_id 列出（created_at asc）。
		ListPythonExecAuditsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentPythonExec, error)
	}

	customAgentPythonExecsModel struct {
		*defaultAgentPythonExecsModel
	}

	agentPythonExecAuditRow struct {
		PythonExecID     string       `db:"python_exec_id"`
		RunID            string       `db:"run_id"`
		AgentID          string       `db:"agent_id"`
		SandboxRequestID string       `db:"sandbox_request_id"`
		Status           string       `db:"status"`
		CodeSummary      []byte       `db:"code_summary"`
		ResourceSummary  []byte       `db:"resource_summary"`
		StdoutSummary    []byte       `db:"stdout_summary"`
		StderrSummary    []byte       `db:"stderr_summary"`
		ResultSummary    []byte       `db:"result_summary"`
		ErrorCode        string       `db:"error_code"`
		ErrorMessage     string       `db:"error_message"`
		TraceID          string       `db:"trace_id"`
		RequestID        string       `db:"request_id"`
		StartedAt        time.Time    `db:"started_at"`
		FinishedAt       sql.NullTime `db:"finished_at"`
		CreatedAt        time.Time    `db:"created_at"`
	}
)

// NewAgentPythonExecsModel returns a model for the database table.
func NewAgentPythonExecsModel(conn sqlx.SqlConn) AgentPythonExecsModel {
	return &customAgentPythonExecsModel{
		defaultAgentPythonExecsModel: newAgentPythonExecsModel(conn),
	}
}

func (m *customAgentPythonExecsModel) withSession(session sqlx.Session) AgentPythonExecsModel {
	return NewAgentPythonExecsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentPythonExecsModel) InsertPythonExecAudit(ctx context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
	normalized, err := agentaudit.NormalizeCreatePythonExecInput(input)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	if normalized.PythonExecID == "" {
		normalized.PythonExecID, err = newAgentAuditID()
		if err != nil {
			return agentaudit.AgentPythonExec{}, err
		}
	}
	now := time.Now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	codeSummary, err := marshalAgentAuditSummary(normalized.CodeSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	resourceSummary, err := marshalAgentAuditSummary(normalized.ResourceSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	stdoutSummary, err := marshalAgentAuditSummary(normalized.StdoutSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	stderrSummary, err := marshalAgentAuditSummary(normalized.StderrSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	resultSummary, err := marshalAgentAuditSummary(normalized.ResultSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}

	var row agentPythonExecAuditRow
	err = m.conn.QueryRowCtx(ctx, &row, `
insert into agent_python_execs (
  python_exec_id, run_id, agent_id, sandbox_request_id, status,
  code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1::bigint, $2::bigint, $3::bigint, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
returning python_exec_id::text as python_exec_id, run_id::text as run_id, agent_id::text as agent_id, sandbox_request_id, status,
          code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.PythonExecID, normalized.RunID, normalized.AgentID, normalized.SandboxRequestID, normalized.Status,
		string(codeSummary), string(resourceSummary), string(stdoutSummary), string(stderrSummary), string(resultSummary),
		normalized.ErrorCode, normalized.ErrorMessage, normalized.TraceID, normalized.RequestID, startedAt,
		nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentPythonExec{}, mapAgentAuditInsertError(err, "agent python exec audit already exists", "agent run audit not found")
	}
	return row.toAgentPythonExec()
}

func (m *customAgentPythonExecsModel) FindPythonExecAudit(ctx context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error) {
	pythonExecID = strings.TrimSpace(pythonExecID)
	if pythonExecID == "" {
		return agentaudit.AgentPythonExec{}, apperror.InvalidArgument("python_exec_id is required")
	}
	var row agentPythonExecAuditRow
	err := m.conn.QueryRowCtx(ctx, &row, `
select python_exec_id::text as python_exec_id, run_id::text as run_id, agent_id::text as agent_id, sandbox_request_id, status,
       code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_python_execs
where python_exec_id = $1::bigint
`, pythonExecID)
	if err != nil {
		if err == ErrNotFound {
			return agentaudit.AgentPythonExec{}, apperror.NotFound("agent python exec audit not found")
		}
		return agentaudit.AgentPythonExec{}, err
	}
	return row.toAgentPythonExec()
}

func (m *customAgentPythonExecsModel) ListPythonExecAuditsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentPythonExec, error) {
	var rows []agentPythonExecAuditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
select python_exec_id::text as python_exec_id, run_id::text as run_id, agent_id::text as agent_id, sandbox_request_id, status,
       code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_python_execs
where run_id = $1::bigint
order by created_at asc, python_exec_id asc
`, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	execs := make([]agentaudit.AgentPythonExec, 0, len(rows))
	for _, row := range rows {
		exec, err := row.toAgentPythonExec()
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	return execs, nil
}

func (r agentPythonExecAuditRow) toAgentPythonExec() (agentaudit.AgentPythonExec, error) {
	codeSummary, err := unmarshalAgentAuditSummary(r.CodeSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	resourceSummary, err := unmarshalAgentAuditSummary(r.ResourceSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	stdoutSummary, err := unmarshalAgentAuditSummary(r.StdoutSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	stderrSummary, err := unmarshalAgentAuditSummary(r.StderrSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	resultSummary, err := unmarshalAgentAuditSummary(r.ResultSummary)
	if err != nil {
		return agentaudit.AgentPythonExec{}, err
	}
	return agentaudit.AgentPythonExec{
		PythonExecID:     r.PythonExecID,
		RunID:            r.RunID,
		AgentID:          r.AgentID,
		SandboxRequestID: r.SandboxRequestID,
		Status:           agentaudit.Status(r.Status),
		CodeSummary:      codeSummary,
		ResourceSummary:  resourceSummary,
		StdoutSummary:    stdoutSummary,
		StderrSummary:    stderrSummary,
		ResultSummary:    resultSummary,
		ErrorCode:        r.ErrorCode,
		ErrorMessage:     r.ErrorMessage,
		TraceID:          r.TraceID,
		RequestID:        r.RequestID,
		StartedAt:        r.StartedAt.UTC(),
		FinishedAt:       nullTimeUTC(r.FinishedAt),
		CreatedAt:        r.CreatedAt.UTC(),
	}, nil
}
