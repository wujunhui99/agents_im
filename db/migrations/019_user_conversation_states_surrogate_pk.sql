-- user_conversation_states: 引入自增代理主键 id，复合 (account_id, conversation_id) 降为唯一约束。
-- 目的：goctl model pg 不支持复合主键，改为单列代理主键后 msg-rpc 可原生生成 user_conversation_states model。
-- 向后兼容：保留 (account_id, conversation_id) 唯一性，message monolith / gateway-ws 的列名 SELECT 与
--           ON CONFLICT (account_id, conversation_id) upsert 不受影响；新增列对既有 SELECT 透明。

-- 1) 复合主键降级为唯一约束（保留唯一性，供 ON CONFLICT 使用）
ALTER TABLE user_conversation_states DROP CONSTRAINT IF EXISTS user_conversation_states_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'user_conversation_states'::regclass
      AND conname = 'user_conversation_states_account_conversation_key'
  ) THEN
    ALTER TABLE user_conversation_states
      ADD CONSTRAINT user_conversation_states_account_conversation_key UNIQUE (account_id, conversation_id);
  END IF;
END $$;

-- 2) 新增自增代理主键 id（既有行自动回填）
ALTER TABLE user_conversation_states ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'user_conversation_states'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE user_conversation_states ADD PRIMARY KEY (id);
  END IF;
END $$;
