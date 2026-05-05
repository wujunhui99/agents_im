# YYYY-MM-DD Short Database Change Description

## Purpose

Describe why this database change is needed.

## Impacted tables / fields / indexes

- Table:
- Fields/indexes:

## Destructive?

- Yes/No:
- If yes, describe data loss or compatibility impact.

## Apply order

1. Apply SQL:
   ```bash
   psql -v ON_ERROR_STOP=1 -f db/change_log/YYYY-MM-DD-short-description.sql
   ```
2. Deploy application changes that depend on it, if any.

## Rollback / recovery

Describe rollback SQL, restore-from-backup strategy, or why rollback is not safely automatic.

## Verification

```bash
git diff --check
go test ./...
bash scripts/verify-static.sh
AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
```
