package repository

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPostgresTaskReportRepositoryUpsertAndReadBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresTaskReportRepositoryFromConn(sqlx.NewSqlConnFromDB(db))

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO task_reports")).
		WithArgs(
			"codex-issue-239", "codex", "sess-123", int64(239), "https://github.com/wujunhui99/agents_im/issues/239", "wujunhui99/agents_im",
			"feat/hermes/issue-239-task-report-management", "/tmp/worktree", "abcdef123456", "success", "2026-05-26T01:00:00Z", "2026-05-26T01:10:00Z",
			int64(600), int64(12345), "https://github.com/wujunhui99/agents_im/pull/240", `["go test ./..."]`, `["none"]`, `["review"]`,
			"no", `["database"]`, "permissions were sufficient", `["keep reports visible"]`, "done", "2026-05-26T01:11:00Z",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("select task_id, agent, codex_session_id")).
		WithArgs("codex-issue-239").
		WillReturnRows(taskReportRows().AddRow(
			"codex-issue-239", "codex", "sess-123", int64(239), "https://github.com/wujunhui99/agents_im/issues/239", "wujunhui99/agents_im",
			"feat/hermes/issue-239-task-report-management", "/tmp/worktree", "abcdef123456", "success", "2026-05-26 01:00:00+00", "2026-05-26 01:10:00+00",
			int64(600), int64(12345), "https://github.com/wujunhui99/agents_im/pull/240", []byte(`["go test ./..."]`), []byte(`["none"]`), []byte(`["review"]`),
			"no", []byte(`["database"]`), "permissions were sufficient", []byte(`["keep reports visible"]`), "done", "2026-05-26 01:11:00+00",
		))

	report, err := repo.UpsertTaskReport(context.Background(), TaskReport{
		TaskID:                  "codex-issue-239",
		Agent:                   "codex",
		CodexSessionID:          "sess-123",
		IssueNumber:             239,
		IssueURL:                "https://github.com/wujunhui99/agents_im/issues/239",
		Repo:                    "wujunhui99/agents_im",
		Branch:                  "feat/hermes/issue-239-task-report-management",
		Worktree:                "/tmp/worktree",
		Commit:                  "abcdef123456",
		Outcome:                 "success",
		StartedAt:               "2026-05-26T01:00:00Z",
		EndedAt:                 "2026-05-26T01:10:00Z",
		DurationSeconds:         600,
		TokensUsed:              12345,
		PRURL:                   "https://github.com/wujunhui99/agents_im/pull/240",
		Evidence:                []string{"go test ./..."},
		Blockers:                []string{"none"},
		MajorTimeSinks:          []string{"review"},
		WouldMorePermissionHelp: "no",
		CandidatePermissions:    []string{"database"},
		PermissionReason:        "permissions were sufficient",
		PitfallsOrLessons:       []string{"keep reports visible"},
		Notes:                   "done",
		RecordedAt:              "2026-05-26T01:11:00Z",
	})
	if err != nil {
		t.Fatalf("UpsertTaskReport: %v", err)
	}
	if report.TaskID != "codex-issue-239" || report.IssueNumber != 239 || report.Evidence[0] != "go test ./..." || report.CandidatePermissions[0] != "database" {
		t.Fatalf("unexpected report: %+v", report)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTaskReportRepositoryListAppliesOutcomeLimitOffset(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresTaskReportRepositoryFromConn(sqlx.NewSqlConnFromDB(db))
	mock.ExpectQuery(regexp.QuoteMeta("select task_id, agent, codex_session_id")).
		WithArgs("blocked", 25, 5).
		WillReturnRows(taskReportRows().AddRow(
			"codex-issue-239", "codex", "sess-123", int64(239), "https://github.com/wujunhui99/agents_im/issues/239", "wujunhui99/agents_im",
			"branch", "worktree", "commit", "blocked", "", "", int64(0), int64(0), "", []byte(`[]`), []byte(`["missing db"]`), []byte(`[]`),
			"yes", []byte(`["database"]`), "database access would help", []byte(`[]`), "", "2026-05-26 01:11:00+00",
		))

	reports, err := repo.ListTaskReports(context.Background(), TaskReportListFilter{Outcome: "blocked", Limit: 25, Offset: 5})
	if err != nil {
		t.Fatalf("ListTaskReports: %v", err)
	}
	if len(reports) != 1 || reports[0].Outcome != "blocked" || reports[0].Blockers[0] != "missing db" {
		t.Fatalf("unexpected reports: %+v", reports)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresTaskReportRepositoryRejectsBlankTaskID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	repo := NewPostgresTaskReportRepositoryFromConn(sqlx.NewSqlConnFromDB(db))
	_, err = repo.UpsertTaskReport(context.Background(), TaskReport{TaskID: "   "})
	if err == nil || !strings.Contains(err.Error(), "task_id is required") {
		t.Fatalf("expected blank task id error, got %v", err)
	}
}

func taskReportRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"task_id", "agent", "codex_session_id", "issue_number", "issue_url", "repo", "branch", "worktree", "commit_sha",
		"outcome", "started_at", "ended_at", "duration_seconds", "tokens_used", "pr_url", "evidence", "blockers",
		"major_time_sinks", "would_more_permission_help", "candidate_permissions", "permission_reason", "pitfalls_or_lessons", "notes", "recorded_at",
	})
}
