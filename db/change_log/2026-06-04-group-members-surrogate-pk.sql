-- group_members: 引入自增代理主键 id，复合 (group_id, account_id) 降为唯一约束。
-- 与 db/migrations/017_group_members_surrogate_pk.sql 等价；可独立执行。
-- psql -v ON_ERROR_STOP=1 -f db/change_log/2026-06-04-group-members-surrogate-pk.sql

BEGIN;

ALTER TABLE group_members DROP CONSTRAINT IF EXISTS group_members_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'group_members'::regclass
      AND conname = 'group_members_group_account_key'
  ) THEN
    ALTER TABLE group_members
      ADD CONSTRAINT group_members_group_account_key UNIQUE (group_id, account_id);
  END IF;
END $$;

ALTER TABLE group_members ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'group_members'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE group_members ADD PRIMARY KEY (id);
  END IF;
END $$;

COMMIT;
