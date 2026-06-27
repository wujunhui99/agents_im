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

var _ AgentFileReadsModel = (*customAgentFileReadsModel)(nil)

type (
	// AgentFileReadsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customAgentFileReadsModel.
	//
	// agent_file_reads 是 append-only 审计表；只暴露 insert + 只读 custom 方法。bigint keystone：
	// file_read_id/run_id/agent_id/skill_id 库内 bigint，写 `$n::bigint`、读 `col::text`。
	AgentFileReadsModel interface {
		agentFileReadsModel
		withSession(session sqlx.Session) AgentFileReadsModel

		// InsertFileReadAudit 写一条 file read 审计行（file_read_id 空则生成）。
		InsertFileReadAudit(ctx context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error)
		// FindFileReadAudit 按 file_read_id 读；不存在返回 apperror.NotFound。
		FindFileReadAudit(ctx context.Context, fileReadID string) (agentaudit.AgentFileRead, error)
		// ListFileReadAuditsByRunID 按 run_id 列出（created_at asc）。
		ListFileReadAuditsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentFileRead, error)
	}

	customAgentFileReadsModel struct {
		*defaultAgentFileReadsModel
	}

	agentFileReadAuditRow struct {
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
)

// NewAgentFileReadsModel returns a model for the database table.
func NewAgentFileReadsModel(conn sqlx.SqlConn) AgentFileReadsModel {
	return &customAgentFileReadsModel{
		defaultAgentFileReadsModel: newAgentFileReadsModel(conn),
	}
}

func (m *customAgentFileReadsModel) withSession(session sqlx.Session) AgentFileReadsModel {
	return NewAgentFileReadsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customAgentFileReadsModel) InsertFileReadAudit(ctx context.Context, input agentaudit.CreateFileReadInput) (agentaudit.AgentFileRead, error) {
	normalized, err := agentaudit.NormalizeCreateFileReadInput(input)
	if err != nil {
		return agentaudit.AgentFileRead{}, err
	}
	if normalized.FileReadID == "" {
		normalized.FileReadID, err = newAgentAuditID()
		if err != nil {
			return agentaudit.AgentFileRead{}, err
		}
	}
	now := time.Now().UTC()
	startedAt := defaultAuditTime(normalized.StartedAt, now)
	contentSummary, err := marshalAgentAuditSummary(normalized.ContentSummary)
	if err != nil {
		return agentaudit.AgentFileRead{}, err
	}

	var row agentFileReadAuditRow
	err = m.conn.QueryRowCtx(ctx, &row, `
insert into agent_file_reads (
  file_read_id, run_id, agent_id, skill_id, file_id, object_key, sha256,
  status, byte_count, content_summary,
  error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
)
values ($1::bigint, $2::bigint, $3::bigint, $4::bigint, $5, $6, $7, $8, $9, $10::jsonb, $11, $12, $13, $14, $15, $16, $17)
returning file_read_id::text as file_read_id, run_id::text as run_id, agent_id::text as agent_id, skill_id::text as skill_id, file_id, object_key, sha256,
          status, byte_count, content_summary,
          error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
`, normalized.FileReadID, normalized.RunID, normalized.AgentID, normalized.SkillID, normalized.FileID, normalized.ObjectKey,
		normalized.SHA256, normalized.Status, normalized.ByteCount, string(contentSummary), normalized.ErrorCode, normalized.ErrorMessage,
		normalized.TraceID, normalized.RequestID, startedAt, nullableAuditTime(normalized.FinishedAt), now)
	if err != nil {
		return agentaudit.AgentFileRead{}, mapAgentAuditInsertError(err, "agent file read audit already exists", "agent run audit not found")
	}
	return row.toAgentFileRead()
}

func (m *customAgentFileReadsModel) FindFileReadAudit(ctx context.Context, fileReadID string) (agentaudit.AgentFileRead, error) {
	fileReadID = strings.TrimSpace(fileReadID)
	if fileReadID == "" {
		return agentaudit.AgentFileRead{}, apperror.InvalidArgument("file_read_id is required")
	}
	var row agentFileReadAuditRow
	err := m.conn.QueryRowCtx(ctx, &row, `
select file_read_id::text as file_read_id, run_id::text as run_id, agent_id::text as agent_id, skill_id::text as skill_id, file_id, object_key, sha256,
       status, byte_count, content_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_file_reads
where file_read_id = $1::bigint
`, fileReadID)
	if err != nil {
		if err == ErrNotFound {
			return agentaudit.AgentFileRead{}, apperror.NotFound("agent file read audit not found")
		}
		return agentaudit.AgentFileRead{}, err
	}
	return row.toAgentFileRead()
}

func (m *customAgentFileReadsModel) ListFileReadAuditsByRunID(ctx context.Context, runID string) ([]agentaudit.AgentFileRead, error) {
	var rows []agentFileReadAuditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, `
select file_read_id::text as file_read_id, run_id::text as run_id, agent_id::text as agent_id, skill_id::text as skill_id, file_id, object_key, sha256,
       status, byte_count, content_summary,
       error_code, error_message, trace_id, request_id, started_at, finished_at, created_at
from agent_file_reads
where run_id = $1::bigint
order by created_at asc, file_read_id asc
`, strings.TrimSpace(runID)); err != nil {
		return nil, err
	}
	reads := make([]agentaudit.AgentFileRead, 0, len(rows))
	for _, row := range rows {
		read, err := row.toAgentFileRead()
		if err != nil {
			return nil, err
		}
		reads = append(reads, read)
	}
	return reads, nil
}

func (r agentFileReadAuditRow) toAgentFileRead() (agentaudit.AgentFileRead, error) {
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
