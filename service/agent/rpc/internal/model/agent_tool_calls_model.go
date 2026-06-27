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

var _ AgentToolCallsModel = (*customAgentToolCallsModel)(nil)

type (
	// AgentToolCallsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentToolCallsModel.
	//
	// agent_tool_calls 是 append-only 审计表；只暴露 insert + 只读 custom 方法。bigint keystone：
	// tool_call_id/run_id/agent_id/tool_id 库内 bigint（tool_id 可空），写 `$n::bigint`、读 `col::text`。
	AgentToolCallsModel interface {
		agentToolCallsModel
		withSession(session sqlx.Session) AgentToolCallsModel

		// InsertToolCallAudit 写一条 tool call 审计行（tool_call_id 空则生成）。
		InsertToolCallAudit(ctx context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error)
		// FindToolCallAudit 按 tool_call_id 读；不存在返回 apperror.NotFound。
		FindToolCallAudit(ctx context.Context, toolCallID string) (agentaudit.AgentToolCall, error)
		// ListToolCallAuditsByRunID 按 run_id 列出（created_at asc）。
		ListToolCallAuditsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentToolCall, error)
	}

	customAgentToolCallsModel struct {
		*defaultAgentToolCallsModel
	}

	agentToolCallAuditRow struct {
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
)

// NewAgentToolCallsModel returns a model for the database table.
func NewAgentToolCallsModel(conn sqlx.SqlConn) AgentToolCallsModel {
	return &customAgentToolCallsModel{
		defaultAgentToolCallsModel: newAgentToolCallsModel(conn),
	}
}

func (m *customAgentToolCallsModel) withSession(session sqlx.Session) AgentToolCallsModel {
	return NewAgentToolCallsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentToolCallsModel) InsertToolCallAudit(ctx context.Context, input agentaudit.CreateToolCallInput) (agentaudit.AgentToolCall, error) {
	normalized, err := agentaudit.NormalizeCreateToolCallInput(input)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	if normalized.ToolCallID == "" {
		normalized.ToolCallID, err = newAgentAuditID()
		if err != nil {
			return agentaudit.AgentToolCall{}, err
		}
	}
	now := time.Now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	inputSummary, err := marshalAgentAuditSummary(normalized.InputSummary)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}
	outputSummary, err := marshalAgentAuditSummary(normalized.OutputSummary)
	if err != nil {
		return agentaudit.AgentToolCall{}, err
	}

	var row agentToolCallAuditRow
	err = m.conn.QueryRowCtx(ctx, &row, `
insert into agent_tool_calls (
  tool_call_id, run_id, agent_id, tool_id, tool_name, status,
  input_summary, output_summary, duration_ms,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1::bigint, $2::bigint, $3::bigint, nullif($4, '')::bigint, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16)
returning tool_call_id::text as tool_call_id, run_id::text as run_id, agent_id::text as agent_id, coalesce(tool_id::text, '') as tool_id, tool_name, status,
          input_summary, output_summary, duration_ms,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.ToolCallID, normalized.RunID, normalized.AgentID, normalized.ToolID, normalized.ToolName, normalized.Status,
		string(inputSummary), string(outputSummary), normalized.DurationMs, normalized.ErrorCode, normalized.ErrorMessage,
		normalized.TraceID, normalized.RequestID, startedAt, nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentToolCall{}, mapAgentAuditInsertError(err, "agent tool call audit already exists", "agent run audit not found")
	}
	return row.toAgentToolCall()
}

func (m *customAgentToolCallsModel) FindToolCallAudit(ctx context.Context, toolCallID string) (agentaudit.AgentToolCall, error) {
	toolCallID = strings.TrimSpace(toolCallID)
	if toolCallID == "" {
		return agentaudit.AgentToolCall{}, apperror.InvalidArgument("tool_call_id is required")
	}
	var row agentToolCallAuditRow
	err := m.conn.QueryRowCtx(ctx, &row, `
select tool_call_id::text as tool_call_id, run_id::text as run_id, agent_id::text as agent_id, coalesce(tool_id::text, '') as tool_id, tool_name, status,
       input_summary, output_summary, duration_ms,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_tool_calls
where tool_call_id = $1::bigint
`, toolCallID)
	if err != nil {
		if err == ErrNotFound {
			return agentaudit.AgentToolCall{}, apperror.NotFound("agent tool call audit not found")
		}
		return agentaudit.AgentToolCall{}, err
	}
	return row.toAgentToolCall()
}

func (m *customAgentToolCallsModel) ListToolCallAuditsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentToolCall, error) {
	var rows []agentToolCallAuditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
select tool_call_id::text as tool_call_id, run_id::text as run_id, agent_id::text as agent_id, coalesce(tool_id::text, '') as tool_id, tool_name, status,
       input_summary, output_summary, duration_ms,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_tool_calls
where run_id = $1::bigint
order by created_at asc, tool_call_id asc
`, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	calls := make([]agentaudit.AgentToolCall, 0, len(rows))
	for _, row := range rows {
		call, err := row.toAgentToolCall()
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, nil
}

func (r agentToolCallAuditRow) toAgentToolCall() (agentaudit.AgentToolCall, error) {
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
