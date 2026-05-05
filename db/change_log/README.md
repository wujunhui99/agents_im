# Database Change Log

`db/change_log/` records database schema or data-migration changes that must be reviewed and applied intentionally during release.

## When a change log is required

Add a change-log entry only when a PR changes database behavior, for example:

- `db/migrations/*.sql`
- schema/table/index/constraint changes
- data migrations or backfills
- repository SQL changes that require a database shape change
- PostgreSQL integration tests that depend on new schema/data behavior

Do **not** add a change log for pure application code, frontend, docs, or tests that do not change database schema/data behavior.

## Required artifact

The executable `.sql` file is the source of truth. A paired `.md` note is recommended for review context, but the SQL is what release operators apply or verify.

Recommended naming:

```text
db/change_log/YYYY-MM-DD-short-description.sql
db/change_log/YYYY-MM-DD-short-description.md
```

## SQL requirements

- Must be directly executable with `psql -v ON_ERROR_STOP=1 -f <file>.sql`.
- Must not contain secrets, DSNs, passwords, tokens, server host/user/port/key, or production credentials.
- Prefer idempotent SQL when practical.
- If destructive, state that clearly in the paired `.md` and include rollback/recovery notes.

## Markdown note checklist

A paired `.md` note should include:

- Purpose
- Impacted tables/fields/indexes
- Whether the change is destructive
- Apply order
- Rollback/recovery notes
- Verification commands

## Local PostgreSQL integration verification

For database changes, run PostgreSQL integration tests against a dedicated local/test database:

```bash
export DATABASE_URL='postgres://agents_im:[REDACTED]@localhost:5432/agents_im_test?sslmode=disable'
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```

`DATABASE_URL` can also be supplied via `AGENTS_IM_POSTGRES_DSN`. Never point this script at production.
