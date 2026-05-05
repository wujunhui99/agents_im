-- Template for database change-log SQL.
-- Copy this file to db/change_log/YYYY-MM-DD-short-description.sql.
-- The copied SQL file must be executable with:
--   psql -v ON_ERROR_STOP=1 -f db/change_log/YYYY-MM-DD-short-description.sql
--
-- Rules:
-- - Do not include secrets, DSNs, passwords, tokens, or server connection details.
-- - Prefer idempotent SQL when practical.
-- - If destructive, document impact and rollback in the paired .md file.

BEGIN;

-- Example:
-- ALTER TABLE example_table ADD COLUMN IF NOT EXISTS example_column text;

COMMIT;
