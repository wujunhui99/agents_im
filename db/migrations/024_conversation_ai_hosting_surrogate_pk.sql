-- conversation_ai_hosting_settings: 引入自增代理主键 id，复合 (owner_account_id, conversation_id) 降为唯一约束。
-- 目的：goctl model pg 不支持复合主键，改单列代理主键后 agent-rpc 可原生生成 conversation_ai_hosting_settings model
--       （AG-6 ① / D13：agent 数据层脱 internal/repository）。
-- 向后兼容：保留 (owner_account_id, conversation_id) 唯一性，ON CONFLICT (owner_account_id, conversation_id) upsert
--           与既有列名 SELECT 不受影响；新增 id 列对既有 SELECT 透明。
--           per-conversation "仅一方可开启" 的部分唯一索引 conversation_ai_hosting_one_enabled_owner_idx 不变。

-- 1) 复合主键降级为唯一约束（保留唯一性，供 ON CONFLICT 使用）
ALTER TABLE conversation_ai_hosting_settings DROP CONSTRAINT IF EXISTS conversation_ai_hosting_settings_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'conversation_ai_hosting_settings'::regclass
      AND conname = 'conversation_ai_hosting_owner_conversation_key'
  ) THEN
    ALTER TABLE conversation_ai_hosting_settings
      ADD CONSTRAINT conversation_ai_hosting_owner_conversation_key UNIQUE (owner_account_id, conversation_id);
  END IF;
END $$;

-- 2) 新增自增代理主键 id（既有行自动回填）
ALTER TABLE conversation_ai_hosting_settings ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'conversation_ai_hosting_settings'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE conversation_ai_hosting_settings ADD PRIMARY KEY (id);
  END IF;
END $$;
