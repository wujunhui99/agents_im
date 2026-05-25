-- Feedback submissions for beta users and admin triage.

CREATE TABLE IF NOT EXISTS feedback (
    feedback_id BIGINT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES accounts(account_id) ON DELETE CASCADE,
    category TEXT NOT NULL CHECK (category IN ('bug', 'poor_experience', 'feature_request', 'other')),
    status TEXT NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'triaged', 'planned', 'resolved', 'rejected')),
    title TEXT NOT NULL CHECK (length(trim(title)) > 0 AND length(title) <= 200),
    content TEXT NOT NULL CHECK (length(trim(content)) > 0 AND length(content) <= 10000),
    contact TEXT,
    page_url TEXT,
    user_agent TEXT,
    client_meta JSONB NOT NULL DEFAULT '{}'::jsonb,
    admin_note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_feedback_status_created_at ON feedback (status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_feedback_user_created_at ON feedback (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_feedback_created_at ON feedback (created_at DESC);
