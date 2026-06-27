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

var _ AgentRunsModel = (*customAgentRunsModel)(nil)

type (
	// AgentRunsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentRunsModel.
	//
	// agent_runs 是 append-only 审计表（migration 002 触发器拒 UPDATE/DELETE），故只暴露 insert +
	// 只读 custom 方法，goctl 生成的 Update/Delete 不使用。bigint keystone：run_id/agent_id 库内 bigint，
	// 域内 string，写 `$n::bigint`、读 `col::text`（#013/#550）。
	AgentRunsModel interface {
		agentRunsModel
		withSession(session sqlx.Session) AgentRunsModel

		// InsertRunAudit 写一条 run 审计行（run_id 空则用 newAgentAuditID 生成），返回写入后的行。
		InsertRunAudit(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error)
		// FindRunAudit 按 run_id 读 run 审计行；不存在返回 apperror.NotFound。
		FindRunAudit(ctx context.Context, runID string) (agentaudit.AgentRun, error)
		// FindRunAuditByTraceID 取该 trace_id 下最新的 run 审计行；不存在返回 apperror.NotFound。
		FindRunAuditByTraceID(ctx context.Context, traceID string) (agentaudit.AgentRun, error)
		// ListRunAudits 按可选 status 过滤、created_at desc 分页列出 run 审计行。
		ListRunAudits(ctx context.Context, status string, limit int, offset int) ([]agentaudit.AgentRun, error)
		// CountRunAudits 统计 run 审计行数（status 空则统计全部）。
		CountRunAudits(ctx context.Context, status string) (int64, error)
	}

	customAgentRunsModel struct {
		*defaultAgentRunsModel
	}

	agentRunAuditRow struct {
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
)

// NewAgentRunsModel returns a model for the database table.
func NewAgentRunsModel(conn sqlx.SqlConn) AgentRunsModel {
	return &customAgentRunsModel{
		defaultAgentRunsModel: newAgentRunsModel(conn),
	}
}

func (m *customAgentRunsModel) withSession(session sqlx.Session) AgentRunsModel {
	return NewAgentRunsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentRunsModel) InsertRunAudit(ctx context.Context, input agentaudit.CreateRunInput) (agentaudit.AgentRun, error) {
	normalized, err := agentaudit.NormalizeCreateRunInput(input)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	if normalized.RunID == "" {
		normalized.RunID, err = newAgentAuditID()
		if err != nil {
			return agentaudit.AgentRun{}, err
		}
	}
	now := time.Now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	inputSummary, err := marshalAgentAuditSummary(normalized.InputSummary)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}
	outputSummary, err := marshalAgentAuditSummary(normalized.OutputSummary)
	if err != nil {
		return agentaudit.AgentRun{}, err
	}

	var row agentRunAuditRow
	err = m.conn.QueryRowCtx(ctx, &row, `
insert into agent_runs (
  run_id, agent_id, conversation_id, trigger_message_id, requesting_user_id,
  status, input_summary, output_summary, output_message_id,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1::bigint, $2::bigint, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16)
returning run_id::text as run_id, agent_id::text as agent_id, conversation_id, trigger_message_id, requesting_user_id,
          status, input_summary, output_summary, output_message_id,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.RunID, normalized.AgentID, normalized.ConversationID, normalized.TriggerMessageID, normalized.RequestingUserID,
		normalized.Status, string(inputSummary), string(outputSummary), normalized.OutputMessageID, normalized.ErrorCode, normalized.ErrorMessage,
		normalized.TraceID, normalized.RequestID, startedAt, nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentRun{}, mapAgentAuditInsertError(err, "agent run audit already exists", "agent run audit not found")
	}
	return row.toAgentRun()
}

func (m *customAgentRunsModel) FindRunAudit(ctx context.Context, runID string) (agentaudit.AgentRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("run_id is required")
	}
	var row agentRunAuditRow
	err := m.conn.QueryRowCtx(ctx, &row, `
select run_id::text as run_id, agent_id::text as agent_id, conversation_id, trigger_message_id, requesting_user_id,
       status, input_summary, output_summary, output_message_id,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_runs
where run_id = $1::bigint
`, runID)
	if err != nil {
		if err == ErrNotFound {
			return agentaudit.AgentRun{}, apperror.NotFound("agent run audit not found")
		}
		return agentaudit.AgentRun{}, err
	}
	return row.toAgentRun()
}

func (m *customAgentRunsModel) FindRunAuditByTraceID(ctx context.Context, traceID string) (agentaudit.AgentRun, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return agentaudit.AgentRun{}, apperror.InvalidArgument("trace_id is required")
	}
	var row agentRunAuditRow
	err := m.conn.QueryRowCtx(ctx, &row, `
select run_id::text as run_id, agent_id::text as agent_id, conversation_id, trigger_message_id, requesting_user_id,
       status, input_summary, output_summary, output_message_id,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_runs
where trace_id = $1
order by created_at desc, run_id asc
limit 1
`, traceID)
	if err != nil {
		if err == ErrNotFound {
			return agentaudit.AgentRun{}, apperror.NotFound("agent run audit not found")
		}
		return agentaudit.AgentRun{}, err
	}
	return row.toAgentRun()
}

func (m *customAgentRunsModel) ListRunAudits(ctx context.Context, status string, limit int, offset int) ([]agentaudit.AgentRun, error) {
	limit = normalizeAgentAuditLimit(limit, 20, 100)
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := ""
	status = strings.TrimSpace(status)
	if status != "" {
		args = append(args, status)
		where = "where status = $" + agentAuditItoa(len(args))
	}
	args = append(args, limit, offset)
	query := `
select run_id::text as run_id, agent_id::text as agent_id, conversation_id, trigger_message_id, requesting_user_id,
       status, input_summary, output_summary, output_message_id,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_runs
` + where + `
order by created_at desc, run_id asc
limit $` + agentAuditItoa(len(args)-1) + ` offset $` + agentAuditItoa(len(args)) + `
`
	var rows []agentRunAuditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	runs := make([]agentaudit.AgentRun, 0, len(rows))
	for _, row := range rows {
		run, err := row.toAgentRun()
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (m *customAgentRunsModel) CountRunAudits(ctx context.Context, status string) (int64, error) {
	status = strings.TrimSpace(status)
	var count int64
	if status == "" {
		err := m.conn.QueryRowCtx(ctx, &count, `select count(*) from agent_runs`)
		return count, err
	}
	err := m.conn.QueryRowCtx(ctx, &count, `select count(*) from agent_runs where status = $1`, status)
	return count, err
}

func (r agentRunAuditRow) toAgentRun() (agentaudit.AgentRun, error) {
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
