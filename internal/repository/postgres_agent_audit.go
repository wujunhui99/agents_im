package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/wujunhui99/agents_im/internal/apperror"
	"github.com/wujunhui99/agents_im/internal/domain/agentaudit"
	"github.com/zeromicro/go-zero/core/stores/postgres"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ AgentAuditRepository = (*PostgresAgentAuditRepository)(nil)

type PostgresAgentAuditRepository struct {
	conn  sqlx.SqlConn
	now   func() time.Time
	newID func(prefix string) (string, error)
}

type postgresAgentRunAuditRow struct {
	RunID            string       `db:"run_id"`
	AgentID          string       `db:"agent_id"`
	ConversationID   string       `db:"conversation_id"`
	TriggerMessageID string       `db:"trigger_message_id"`
	RequestingUserID string       `db:"requesting_user_id"`
	Status           string       `db:"status"`
	InputSummary     []byte       `db:"input_summary"`
	OutputSummary    []byte       `db:"output_summary"`
	OutputMessageID  string       `db:"output_message_id"`
	ErrorCode        string       `db:"error_code"`
	ErrorMessage     string       `db:"error_message"`
	TraceID          string       `db:"trace_id"`
	RequestID        string       `db:"request_id"`
	StartedAt        time.Time    `db:"started_at"`
	FinishedAt       sql.NullTime `db:"finished_at"`
	CreatedAt        time.Time    `db:"created_at"`
}

type postgresAgentToolCallAuditRow struct {
	ToolCallID    string       `db:"tool_call_id"`
	RunID         string       `db:"run_id"`
	AgentID       string       `db:"agent_id"`
	ToolID        string       `db:"tool_id"`
	ToolName      string       `db:"tool_name"`
	Status        string       `db:"status"`
	InputSummary  []byte       `db:"input_summary"`
	OutputSummary []byte       `db:"output_summary"`
	DurationMs    int64        `db:"duration_ms"`
	ErrorCode     string       `db:"error_code"`
	ErrorMessage  string       `db:"error_message"`
	TraceID       string       `db:"trace_id"`
	RequestID     string       `db:"request_id"`
	StartedAt     time.Time    `db:"started_at"`
	FinishedAt    sql.NullTime `db:"finished_at"`
	CreatedAt     time.Time    `db:"created_at"`
}

type postgresAgentFileReadAuditRow struct {
	FileReadID     string       `db:"file_read_id"`
	RunID          string       `db:"run_id"`
	AgentID        string       `db:"agent_id"`
	SkillID        string       `db:"skill_id"`
	FileID         string       `db:"file_id"`
	ObjectKey      string       `db:"object_key"`
	SHA256         string       `db:"sha256"`
	Status         string       `db:"status"`
	ByteCount      int64        `db:"byte_count"`
	ContentSummary []byte       `db:"content_summary"`
	ErrorCode      string       `db:"error_code"`
	ErrorMessage   string       `db:"error_message"`
	TraceID        string       `db:"trace_id"`
	RequestID      string       `db:"request_id"`
	StartedAt      time.Time    `db:"started_at"`
	FinishedAt     sql.NullTime `db:"finished_at"`
	CreatedAt      time.Time    `db:"created_at"`
}

type postgresAgentPythonExecAuditRow struct {
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

func NewPostgresAgentAuditRepository(dataSource string) (*PostgresAgentAuditRepository, error) {
	dataSource = strings.TrimSpace(dataSource)
	if dataSource == "" {
		return nil, sql.ErrConnDone
	}
	return NewPostgresAgentAuditRepositoryFromConn(postgres.New(dataSource)), nil
}

func NewPostgresAgentAuditRepositoryFromConn(conn sqlx.SqlConn) *PostgresAgentAuditRepository {
	return &PostgresAgentAuditRepository{conn: conn, now: time.Now, newID: newAuditID}
}

func (r *PostgresAgentAuditRepository) CreateAgentRun(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
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
	now := r.now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	inputSummary, err := marshalAgentAuditSummary(normalized.InputSummary)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	outputSummary, err := marshalAgentAuditSummary(normalized.OutputSummary)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}

	var row postgresAgentRunAuditRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_runs (
  run_id, agent_id, conversation_id, trigger_message_id, requesting_user_id,
  status, input_summary, output_summary, output_message_id,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16)
returning run_id, agent_id, conversation_id, trigger_message_id, requesting_user_id,
          status, input_summary, output_summary, output_message_id,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.RunID, normalized.AgentID, normalized.ConversationID, normalized.TriggerMessageID, normalized.RequestingUserID,
		normalized.Status, string(inputSummary), string(outputSummary), normalized.OutputMessageID, normalized.ErrorCode, normalized.ErrorMessage,
		normalized.TraceID, normalized.RequestID, startedAt, nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentRun{}, mapAgentAuditInsertError(err, "agent run audit already exists", "agent run audit not found")
	}
	return row.agentRun()
}

func (r *PostgresAgentAuditRepository) GetAgentRun(ctx context.Context, runID string) (agentaudit.AgentRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("run_id is required")
	}
	var row postgresAgentRunAuditRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select run_id, agent_id, conversation_id, trigger_message_id, requesting_user_id,
       status, input_summary, output_summary, output_message_id,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_runs
where run_id = $1
`, runID)
	if err != nil {
		if isNotFound(err) {
			return agentaudit.AgentRun{}, apperror.NotFound("agent run audit not found")
		}
		return agentaudit.AgentRun{}, err
	}
	return row.agentRun()
}

func (r *PostgresAgentAuditRepository) CreateAgentToolCall(ctx context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
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
	now := r.now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	inputSummary, err := marshalAgentAuditSummary(normalized.InputSummary)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	outputSummary, err := marshalAgentAuditSummary(normalized.OutputSummary)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}

	var row postgresAgentToolCallAuditRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_tool_calls (
  tool_call_id, run_id, agent_id, tool_id, tool_name, status,
  input_summary, output_summary, duration_ms,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16)
returning tool_call_id, run_id, agent_id, tool_id, tool_name, status,
          input_summary, output_summary, duration_ms,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.ToolCallID, normalized.RunID, normalized.AgentID, normalized.ToolID, normalized.ToolName, normalized.Status,
		string(inputSummary), string(outputSummary), normalized.DurationMs, normalized.ErrorCode, normalized.ErrorMessage,
		normalized.TraceID, normalized.RequestID, startedAt, nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentToolCall{}, mapAgentAuditInsertError(err, "agent tool call audit already exists", "agent run audit not found")
	}
	return row.agentToolCall()
}

func (r *PostgresAgentAuditRepository) GetAgentToolCall(ctx context.Context, toolCallID string) (agentaudit.AgentToolCall, error) {
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		return agentaudit.AgentToolCall{}, apperror.InvalidArgument("tool_call_id is required")
	}
	var row postgresAgentToolCallAuditRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select tool_call_id, run_id, agent_id, tool_id, tool_name, status,
       input_summary, output_summary, duration_ms,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_tool_calls
where tool_call_id = $1
`, toolCallID)
	if err != nil {
		if isNotFound(err) {
			return agentaudit.AgentToolCall{}, apperror.NotFound("agent tool call audit not found")
		}
		return agentaudit.AgentToolCall{}, err
	}
	return row.agentToolCall()
}

func (r *PostgresAgentAuditRepository) ListAgentToolCallsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentToolCall, error) {
	if _, err := r.GetAgentRun(ctx, runID); err != nil {
		return nil, err
	}
	var rows []postgresAgentToolCallAuditRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
select tool_call_id, run_id, agent_id, tool_id, tool_name, status,
       input_summary, output_summary, duration_ms,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_tool_calls
where run_id = $1
order by created_at asc, tool_call_id asc
`, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	calls := make([]agentaudit.AgentToolCall, 0, len(rows))
	for _, row := range rows {
		call, err := row.agentToolCall()
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func (r *PostgresAgentAuditRepository) CreateAgentFileRead(ctx context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
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
	now := r.now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	contentSummary, err := marshalAgentAuditSummary(normalized.ContentSummary)
	if err != nil {
		return agentaudit.AgentFileRead{}, err
	}

	var row postgresAgentFileReadAuditRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_file_reads (
  file_read_id, run_id, agent_id, skill_id, file_id, object_key, sha256,
  status, byte_count, content_summary,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
returning file_read_id, run_id, agent_id, skill_id, file_id, object_key, sha256,
          status, byte_count, content_summary,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.FileReadID, normalized.RunID, normalized.AgentID, normalized.SkillID, normalized.FileID, normalized.ObjectKey,
		normalized.SHA256, normalized.Status, normalized.ByteCount, string(contentSummary), normalized.ErrorCode, normalized.ErrorMessage,
		normalized.TraceID, normalized.RequestID, startedAt, nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentFileRead{}, mapAgentAuditInsertError(err, "agent file read audit already exists", "agent run audit not found")
	}
	return row.agentFileRead()
}

func (r *PostgresAgentAuditRepository) GetAgentFileRead(ctx context.Context, fileReadID string) (agentaudit.AgentFileRead, error) {
	fileReadID = strings.TrimSpace(fileReadID)
	if fileReadID == "" {
		return agentaudit.AgentFileRead{}, apperror.InvalidArgument("file_read_id is required")
	}
	var row postgresAgentFileReadAuditRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select file_read_id, run_id, agent_id, skill_id, file_id, object_key, sha256,
       status, byte_count, content_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_file_reads
where file_read_id = $1
`, fileReadID)
	if err != nil {
		if isNotFound(err) {
			return agentaudit.AgentFileRead{}, apperror.NotFound("agent file read audit not found")
		}
		return agentaudit.AgentFileRead{}, err
	}
	return row.agentFileRead()
}

func (r *PostgresAgentAuditRepository) ListAgentFileReadsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentFileRead, error) {
	if _, err := r.GetAgentRun(ctx, runID); err != nil {
		return nil, err
	}
	var rows []postgresAgentFileReadAuditRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
select file_read_id, run_id, agent_id, skill_id, file_id, object_key, sha256,
       status, byte_count, content_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_file_reads
where run_id = $1
order by created_at asc, file_read_id asc
`, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	reads := make([]agentaudit.AgentFileRead, 0, len(rows))
	for _, row := range rows {
		read, err := row.agentFileRead()
		if err != nil {
			return nil, err
		}
		reads = append(reads, read)
	}
	return reads, nil
}

func (r *PostgresAgentAuditRepository) CreateAgentPythonExec(ctx context.Context, input agentaudit.CreatePythonExecInput) (agentaudit.AgentPythonExec, error) {
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
	now := r.now().UTC()
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

	var row postgresAgentPythonExecAuditRow
	err = r.conn.QueryRowCtx(ctx, &row, `
insert into agent_python_execs (
  python_exec_id, run_id, agent_id, sandbox_request_id, status,
  code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
returning python_exec_id, run_id, agent_id, sandbox_request_id, status,
          code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.PythonExecID, normalized.RunID, normalized.AgentID, normalized.SandboxRequestID, normalized.Status,
		string(codeSummary), string(resourceSummary), string(stdoutSummary), string(stderrSummary), string(resultSummary),
		normalized.ErrorCode, normalized.ErrorMessage, normalized.TraceID, normalized.RequestID, startedAt,
		nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentPythonExec{}, mapAgentAuditInsertError(err, "agent python exec audit already exists", "agent run audit not found")
	}
	return row.agentPythonExec()
}

func (r *PostgresAgentAuditRepository) GetAgentPythonExec(ctx context.Context, pythonExecID string) (agentaudit.AgentPythonExec, error) {
	pythonExecID = strings.TrimSpace(pythonExecID)
	if pythonExecID == "" {
		return agentaudit.AgentPythonExec{}, apperror.InvalidArgument("python_exec_id is required")
	}
	var row postgresAgentPythonExecAuditRow
	err := r.conn.QueryRowCtx(ctx, &row, `
select python_exec_id, run_id, agent_id, sandbox_request_id, status,
       code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_python_execs
where python_exec_id = $1
`, pythonExecID)
	if err != nil {
		if isNotFound(err) {
			return agentaudit.AgentPythonExec{}, apperror.NotFound("agent python exec audit not found")
		}
		return agentaudit.AgentPythonExec{}, err
	}
	return row.agentPythonExec()
}

func (r *PostgresAgentAuditRepository) ListAgentPythonExecsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentPythonExec, error) {
	if _, err := r.GetAgentRun(ctx, runID); err != nil {
		return nil, err
	}
	var rows []postgresAgentPythonExecAuditRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, `
select python_exec_id, run_id, agent_id, sandbox_request_id, status,
       code_summary, resource_summary, stdout_summary, stderr_summary, result_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_python_execs
where run_id = $1
order by created_at asc, python_exec_id asc
`, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	execs := make([]agentaudit.AgentPythonExec, 0, len(rows))
	for _, row := range rows {
		exec, err := row.agentPythonExec()
		if err != nil {
			return nil, err
		}
		execs = append(execs, exec)
	}
	return execs, nil
}

func (r postgresAgentRunAuditRow) agentRun() (agentaudit.AgentRun, error) {
	inputSummary, err := unmarshalAgentAuditSummary(r.InputSummary)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	outputSummary, err := unmarshalAgentAuditSummary(r.OutputSummary)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	return agentaudit.AgentRun{
		RunID:            r.RunID,
		AgentID:          r.AgentID,
		ConversationID:   r.ConversationID,
		TriggerMessageID: r.TriggerMessageID,
		RequestingUserID: r.RequestingUserID,
		Status:           agentaudit.Status(r.Status),
		InputSummary:     inputSummary,
		OutputSummary:    outputSummary,
		OutputMessageID:  r.OutputMessageID,
		ErrorCode:        r.ErrorCode,
		ErrorMessage:     r.ErrorMessage,
		TraceID:          r.TraceID,
		RequestID:        r.RequestID,
		StartedAt:        r.StartedAt.UTC(),
		FinishedAt:       nullTimeUTC(r.FinishedAt),
		CreatedAt:        r.CreatedAt.UTC(),
	}, nil
}

func (r postgresAgentToolCallAuditRow) agentToolCall() (agentaudit.AgentToolCall, error) {
	inputSummary, err := unmarshalAgentAuditSummary(r.InputSummary)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	outputSummary, err := unmarshalAgentAuditSummary(r.OutputSummary)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	return agentaudit.AgentToolCall{
		ToolCallID:    r.ToolCallID,
		RunID:         r.RunID,
		AgentID:       r.AgentID,
		ToolID:        r.ToolID,
		ToolName:      r.ToolName,
		Status:        agentaudit.Status(r.Status),
		InputSummary:  inputSummary,
		OutputSummary: outputSummary,
		DurationMs:    r.DurationMs,
		ErrorCode:     r.ErrorCode,
		ErrorMessage:  r.ErrorMessage,
		TraceID:       r.TraceID,
		RequestID:     r.RequestID,
		StartedAt:     r.StartedAt.UTC(),
		FinishedAt:    nullTimeUTC(r.FinishedAt),
		CreatedAt:     r.CreatedAt.UTC(),
	}, nil
}

func (r postgresAgentFileReadAuditRow) agentFileRead() (agentaudit.AgentFileRead, error) {
	contentSummary, err := unmarshalAgentAuditSummary(r.ContentSummary)
	if err != nil {
		return agentaudit.AgentFileRead{}, err
	}
	return agentaudit.AgentFileRead{
		FileReadID:     r.FileReadID,
		RunID:          r.RunID,
		AgentID:        r.AgentID,
		SkillID:        r.SkillID,
		FileID:         r.FileID,
		ObjectKey:      r.ObjectKey,
		SHA256:         r.SHA256,
		Status:         agentaudit.Status(r.Status),
		ByteCount:      r.ByteCount,
		ContentSummary: contentSummary,
		ErrorCode:      r.ErrorCode,
		ErrorMessage:   r.ErrorMessage,
		TraceID:        r.TraceID,
		RequestID:      r.RequestID,
		StartedAt:      r.StartedAt.UTC(),
		FinishedAt:     nullTimeUTC(r.FinishedAt),
		CreatedAt:      r.CreatedAt.UTC(),
	}, nil
}

func (r postgresAgentPythonExecAuditRow) agentPythonExec() (agentaudit.AgentPythonExec, error) {
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

func marshalAgentAuditSummary(summary agentaudit.Summary) ([]byte, error) {
	redacted, err := agentaudit.RedactSummary(summary)
	if err != nil {
		return nil, err
	}
	if redacted == nil {
		redacted = agentaudit.Summary{}
	}
	return json.Marshal(redacted)
}

func unmarshalAgentAuditSummary(raw []byte) (agentaudit.Summary, error) {
	if len(raw) == 0 {
		return agentaudit.Summary{}, nil
	}
	var summary map[string]any
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, err
	}
	return agentaudit.Summary(summary), nil
}

func nullableAuditTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullTimeUTC(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}
	return value.Time.UTC()
}

func mapAgentAuditInsertError(err error, alreadyExistsMessage string, notFoundMessage string) error {
	if isPostgresUniqueViolation(err) {
		return apperror.AlreadyExists(alreadyExistsMessage)
	}
	if isPostgresForeignKeyViolation(err) {
		return apperror.NotFound(notFoundMessage)
	}
	if isPostgresCheckViolation(err) {
		return apperror.InvalidArgument("invalid agent audit record")
	}
	return err
}
