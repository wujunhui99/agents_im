-- Hermes/Codex controller task reports for Management System task tracking.

CREATE TABLE IF NOT EXISTS task_reports (
    task_id TEXT PRIMARY KEY,
    agent TEXT NOT NULL DEFAULT '',
    codex_session_id TEXT NOT NULL DEFAULT '',
    issue_number BIGINT,
    issue_url TEXT NOT NULL DEFAULT '',
    repo TEXT NOT NULL DEFAULT '',
    branch TEXT NOT NULL DEFAULT '',
    worktree TEXT NOT NULL DEFAULT '',
    commit_sha TEXT NOT NULL DEFAULT '',
    outcome TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_seconds BIGINT,
    tokens_used BIGINT,
    pr_url TEXT NOT NULL DEFAULT '',
    evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
    blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    major_time_sinks JSONB NOT NULL DEFAULT '[]'::jsonb,
    would_more_permission_help TEXT NOT NULL DEFAULT '',
    candidate_permissions JSONB NOT NULL DEFAULT '[]'::jsonb,
    permission_reason TEXT NOT NULL DEFAULT '',
    pitfalls_or_lessons JSONB NOT NULL DEFAULT '[]'::jsonb,
    notes TEXT NOT NULL DEFAULT '',
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT task_reports_task_id_not_blank CHECK (task_id <> '')
);

CREATE INDEX IF NOT EXISTS idx_task_reports_recorded_at ON task_reports (recorded_at DESC, task_id DESC);
CREATE INDEX IF NOT EXISTS idx_task_reports_outcome_recorded_at ON task_reports (outcome, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_reports_issue_number ON task_reports (issue_number) WHERE issue_number IS NOT NULL;
