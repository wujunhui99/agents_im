package model

import (
	"context"
	"strconv"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TaskReportsModel = (*customTaskReportsModel)(nil)

type (
	// TaskReportsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customTaskReportsModel.
	TaskReportsModel interface {
		taskReportsModel
		// WithSession 返回绑定到给定事务 session 的 model，供 Logic 层在事务内复用。
		WithSession(session sqlx.Session) TaskReportsModel
		// Transact 暴露事务入口，事务边界由 Logic 层控制（Model 不自行编排业务事务）。
		Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error
		// ListTaskReports returns rows filtered by outcome, newest first.
		ListTaskReports(ctx context.Context, filter TaskReportListFilter) ([]*TaskReports, error)
		// UpsertTaskReport inserts or updates a row by task_id and returns the stored row.
		UpsertTaskReport(ctx context.Context, data *TaskReports) (*TaskReports, error)
	}

	customTaskReportsModel struct {
		*defaultTaskReportsModel
	}

	// TaskReportListFilter narrows ListTaskReports (query spec, not a domain entity).
	TaskReportListFilter struct {
		Outcome string
		Limit   int
		Offset  int
	}
)

// NewTaskReportsModel returns a model for the database table.
func NewTaskReportsModel(conn sqlx.SqlConn) TaskReportsModel {
	return &customTaskReportsModel{
		defaultTaskReportsModel: newTaskReportsModel(conn),
	}
}

func (m *customTaskReportsModel) WithSession(session sqlx.Session) TaskReportsModel {
	return NewTaskReportsModel(sqlx.NewSqlConnFromSession(session))
}

func (m *customTaskReportsModel) Transact(ctx context.Context, fn func(ctx context.Context, session sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, fn)
}

func (m *customTaskReportsModel) ListTaskReports(ctx context.Context, filter TaskReportListFilter) ([]*TaskReports, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args := []any{}
	where := ""
	if filter.Outcome != "" {
		args = append(args, filter.Outcome)
		where = " where outcome = $1"
	}
	args = append(args, limit, offset)
	query := "select " + taskReportsRows + " from " + m.table + where +
		" order by recorded_at desc, task_id desc limit $" + strconv.Itoa(len(args)-1) + " offset $" + strconv.Itoa(len(args))
	var rows []*TaskReports
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *customTaskReportsModel) UpsertTaskReport(ctx context.Context, data *TaskReports) (*TaskReports, error) {
	query := "insert into " + m.table + ` (
  task_id, agent, codex_session_id, issue_number, issue_url, repo, branch, worktree, commit_sha,
  outcome, started_at, ended_at, duration_seconds, tokens_used, pr_url, evidence, blockers,
  major_time_sinks, would_more_permission_help, candidate_permissions, permission_reason,
  pitfalls_or_lessons, notes, recorded_at
) values (
  $1, $2, $3, $4, $5, $6, $7, $8, $9,
  $10, $11, $12, $13, $14, $15, $16::jsonb, $17::jsonb,
  $18::jsonb, $19, $20::jsonb, $21,
  $22::jsonb, $23, $24
)
on conflict (task_id) do update set
  agent = excluded.agent,
  codex_session_id = excluded.codex_session_id,
  issue_number = excluded.issue_number,
  issue_url = excluded.issue_url,
  repo = excluded.repo,
  branch = excluded.branch,
  worktree = excluded.worktree,
  commit_sha = excluded.commit_sha,
  outcome = excluded.outcome,
  started_at = excluded.started_at,
  ended_at = excluded.ended_at,
  duration_seconds = excluded.duration_seconds,
  tokens_used = excluded.tokens_used,
  pr_url = excluded.pr_url,
  evidence = excluded.evidence,
  blockers = excluded.blockers,
  major_time_sinks = excluded.major_time_sinks,
  would_more_permission_help = excluded.would_more_permission_help,
  candidate_permissions = excluded.candidate_permissions,
  permission_reason = excluded.permission_reason,
  pitfalls_or_lessons = excluded.pitfalls_or_lessons,
  notes = excluded.notes,
  recorded_at = excluded.recorded_at`
	if _, err := m.conn.ExecCtx(ctx, query,
		data.TaskId, data.Agent, data.CodexSessionId, data.IssueNumber, data.IssueUrl, data.Repo,
		data.Branch, data.Worktree, data.CommitSha, data.Outcome, data.StartedAt, data.EndedAt,
		data.DurationSeconds, data.TokensUsed, data.PrUrl, data.Evidence, data.Blockers, data.MajorTimeSinks,
		data.WouldMorePermissionHelp, data.CandidatePermissions, data.PermissionReason, data.PitfallsOrLessons, data.Notes,
		data.RecordedAt,
	); err != nil {
		return nil, err
	}
	return m.FindOne(ctx, data.TaskId)
}
