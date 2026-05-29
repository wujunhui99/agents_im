package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/wujunhui99/agents_im/pkg/apperror"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ TaskReportRepository = (*PostgresTaskReportRepository)(nil)

type PostgresTaskReportRepository struct {
	conn sqlx.SqlConn
}

func NewPostgresTaskReportRepository(dataSource string) (*PostgresTaskReportRepository, error) {
	if strings.TrimSpace(dataSource) == "" {
		return nil, apperror.Internal("postgres data source is required")
	}
	return &PostgresTaskReportRepository{conn: sqlx.NewSqlConn("postgres", dataSource)}, nil
}

func NewPostgresTaskReportRepositoryFromConn(conn sqlx.SqlConn) *PostgresTaskReportRepository {
	return &PostgresTaskReportRepository{conn: conn}
}

func (r *PostgresTaskReportRepository) UpsertTaskReport(ctx context.Context, report TaskReport) (TaskReport, error) {
	if r == nil || r.conn == nil {
		return TaskReport{}, apperror.Internal("task report repository is not configured")
	}
	report.TaskID = strings.TrimSpace(report.TaskID)
	if report.TaskID == "" {
		return TaskReport{}, apperror.InvalidArgument("task_id is required")
	}
	evidence, err := json.Marshal(report.Evidence)
	if err != nil {
		return TaskReport{}, err
	}
	blockers, err := json.Marshal(report.Blockers)
	if err != nil {
		return TaskReport{}, err
	}
	timeSinks, err := json.Marshal(report.MajorTimeSinks)
	if err != nil {
		return TaskReport{}, err
	}
	candidatePermissions, err := json.Marshal(report.CandidatePermissions)
	if err != nil {
		return TaskReport{}, err
	}
	lessons, err := json.Marshal(report.PitfallsOrLessons)
	if err != nil {
		return TaskReport{}, err
	}

	query := `
INSERT INTO task_reports (
  task_id, agent, codex_session_id, issue_number, issue_url, repo, branch, worktree, commit_sha,
  outcome, started_at, ended_at, duration_seconds, tokens_used, pr_url, evidence, blockers,
  major_time_sinks, would_more_permission_help, candidate_permissions, permission_reason,
  pitfalls_or_lessons, notes, recorded_at
) VALUES (
  $1, $2, $3, NULLIF($4, 0), $5, $6, $7, $8, $9,
  $10, NULLIF($11, '')::timestamptz, NULLIF($12, '')::timestamptz, NULLIF($13, 0), NULLIF($14, 0), $15, $16::jsonb, $17::jsonb,
  $18::jsonb, $19, $20::jsonb, $21,
  $22::jsonb, $23, coalesce(NULLIF($24, '')::timestamptz, now())
)
ON CONFLICT (task_id) DO UPDATE SET
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
	if _, err := r.conn.ExecCtx(ctx, query,
		report.TaskID, report.Agent, report.CodexSessionID, report.IssueNumber, report.IssueURL, report.Repo,
		report.Branch, report.Worktree, report.Commit, report.Outcome, report.StartedAt, report.EndedAt,
		report.DurationSeconds, report.TokensUsed, report.PRURL, string(evidence), string(blockers), string(timeSinks),
		report.WouldMorePermissionHelp, string(candidatePermissions), report.PermissionReason, string(lessons), report.Notes,
		report.RecordedAt,
	); err != nil {
		return TaskReport{}, err
	}
	return r.getTaskReport(ctx, report.TaskID)
}

func (r *PostgresTaskReportRepository) ListTaskReports(ctx context.Context, filter TaskReportListFilter) ([]TaskReport, error) {
	if r == nil || r.conn == nil {
		return nil, apperror.Internal("task report repository is not configured")
	}
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
	if strings.TrimSpace(filter.Outcome) != "" {
		args = append(args, filter.Outcome)
		where = " where outcome = $1"
	}
	args = append(args, limit, offset)
	limitPos := len(args) - 1
	offsetPos := len(args)
	query := `select task_id, agent, codex_session_id, coalesce(issue_number, 0), issue_url, repo, branch, worktree, commit_sha,
  outcome, coalesce(started_at::text, ''), coalesce(ended_at::text, ''), coalesce(duration_seconds, 0), coalesce(tokens_used, 0), pr_url,
  evidence, blockers, major_time_sinks, would_more_permission_help, candidate_permissions, permission_reason, pitfalls_or_lessons, notes, recorded_at::text
  from task_reports` + where + ` order by recorded_at desc, task_id desc limit $` + intString(limitPos) + ` offset $` + intString(offsetPos)
	var rows []taskReportRow
	if err := r.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	out := make([]TaskReport, 0, len(rows))
	for _, row := range rows {
		report, err := row.toTaskReport()
		if err != nil {
			return nil, err
		}
		out = append(out, report)
	}
	return out, nil
}

func (r *PostgresTaskReportRepository) getTaskReport(ctx context.Context, taskID string) (TaskReport, error) {
	var row taskReportRow
	query := `select task_id, agent, codex_session_id, coalesce(issue_number, 0), issue_url, repo, branch, worktree, commit_sha,
  outcome, coalesce(started_at::text, ''), coalesce(ended_at::text, ''), coalesce(duration_seconds, 0), coalesce(tokens_used, 0), pr_url,
  evidence, blockers, major_time_sinks, would_more_permission_help, candidate_permissions, permission_reason, pitfalls_or_lessons, notes, recorded_at::text
  from task_reports where task_id = $1`
	if err := r.conn.QueryRowCtx(ctx, &row, query, taskID); err != nil {
		if err == sql.ErrNoRows {
			return TaskReport{}, apperror.NotFound("task report not found")
		}
		return TaskReport{}, err
	}
	return row.toTaskReport()
}

type taskReportRow struct {
	TaskID                  string `db:"task_id"`
	Agent                   string `db:"agent"`
	CodexSessionID          string `db:"codex_session_id"`
	IssueNumber             int64  `db:"issue_number"`
	IssueURL                string `db:"issue_url"`
	Repo                    string `db:"repo"`
	Branch                  string `db:"branch"`
	Worktree                string `db:"worktree"`
	Commit                  string `db:"commit_sha"`
	Outcome                 string `db:"outcome"`
	StartedAt               string `db:"started_at"`
	EndedAt                 string `db:"ended_at"`
	DurationSeconds         int64  `db:"duration_seconds"`
	TokensUsed              int64  `db:"tokens_used"`
	PRURL                   string `db:"pr_url"`
	Evidence                []byte `db:"evidence"`
	Blockers                []byte `db:"blockers"`
	MajorTimeSinks          []byte `db:"major_time_sinks"`
	WouldMorePermissionHelp string `db:"would_more_permission_help"`
	CandidatePermissions    []byte `db:"candidate_permissions"`
	PermissionReason        string `db:"permission_reason"`
	PitfallsOrLessons       []byte `db:"pitfalls_or_lessons"`
	Notes                   string `db:"notes"`
	RecordedAt              string `db:"recorded_at"`
}

func (r taskReportRow) toTaskReport() (TaskReport, error) {
	report := TaskReport{
		TaskID:                  r.TaskID,
		Agent:                   r.Agent,
		CodexSessionID:          r.CodexSessionID,
		IssueNumber:             r.IssueNumber,
		IssueURL:                r.IssueURL,
		Repo:                    r.Repo,
		Branch:                  r.Branch,
		Worktree:                r.Worktree,
		Commit:                  r.Commit,
		Outcome:                 r.Outcome,
		StartedAt:               r.StartedAt,
		EndedAt:                 r.EndedAt,
		DurationSeconds:         r.DurationSeconds,
		TokensUsed:              r.TokensUsed,
		PRURL:                   r.PRURL,
		WouldMorePermissionHelp: r.WouldMorePermissionHelp,
		PermissionReason:        r.PermissionReason,
		Notes:                   r.Notes,
		RecordedAt:              r.RecordedAt,
	}
	if err := unmarshalStringSlice(r.Evidence, &report.Evidence); err != nil {
		return TaskReport{}, err
	}
	if err := unmarshalStringSlice(r.Blockers, &report.Blockers); err != nil {
		return TaskReport{}, err
	}
	if err := unmarshalStringSlice(r.MajorTimeSinks, &report.MajorTimeSinks); err != nil {
		return TaskReport{}, err
	}
	if err := unmarshalStringSlice(r.CandidatePermissions, &report.CandidatePermissions); err != nil {
		return TaskReport{}, err
	}
	if err := unmarshalStringSlice(r.PitfallsOrLessons, &report.PitfallsOrLessons); err != nil {
		return TaskReport{}, err
	}
	return report, nil
}

func unmarshalStringSlice(raw []byte, out *[]string) error {
	if len(raw) == 0 {
		*out = []string{}
		return nil
	}
	return json.Unmarshal(raw, out)
}

func intString(value int) string {
	return strconv.Itoa(value)
}
