-- group_members: 引入自增代理主键 id，复合 (group_id, account_id) 降为唯一约束。
-- 目的：goctl model pg 不支持复合主键，改为单列代理主键后可原生生成 group_members model。
-- 向后兼容：保留 (group_id, account_id) 唯一性，message monolith 的列名 SELECT 与
--           ON CONFLICT (group_id, account_id) upsert 不受影响；新增列对既有 SELECT 透明。

-- 1) 复合主键降级为唯一约束（保留唯一性，供 ON CONFLICT 使用）
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

-- 2) 新增自增代理主键 id（既有行自动回填）
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
