-- friendships: 引入自增代理主键 id，复合 (account_id, friend_account_id) 降为唯一约束。
-- 目的：goctl model pg 不支持复合主键，改为单列代理主键后可原生生成 friendships model
--       （friends rpc 数据层脱 internal/repository，详见 #426）。
-- 向后兼容：保留 (account_id, friend_account_id) 唯一性，monolith 的列名 SELECT 与
--           ON CONFLICT (account_id, friend_account_id) upsert（EnsureAcceptedFriendship 等）不受影响；
--           新增列对既有 SELECT 透明。

-- 1) 复合主键降级为唯一约束（保留唯一性，供 ON CONFLICT 使用）
ALTER TABLE friendships DROP CONSTRAINT IF EXISTS friendships_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'friendships'::regclass
      AND conname = 'friendships_account_friend_key'
  ) THEN
    ALTER TABLE friendships
      ADD CONSTRAINT friendships_account_friend_key UNIQUE (account_id, friend_account_id);
  END IF;
END $$;

-- 2) 新增自增代理主键 id（既有行自动回填）
ALTER TABLE friendships ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'friendships'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE friendships ADD PRIMARY KEY (id);
  END IF;
END $$;
