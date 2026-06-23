-- agent_{prompt,tool,skill}_bindings: 引入自增代理主键 id，复合 (agent_id, <ref>_id) 降为唯一约束。
-- 目的：goctl model pg 不支持复合主键，改单列代理主键后 agent-rpc 可原生生成 3 张 binding model
--       （#605：agent 注册表数据层脱 internal/repository → service/agent/rpc/internal/model）。
-- 向后兼容：保留 (agent_id, <ref>_id) 唯一性，ON CONFLICT (agent_id, <ref>_id) upsert 与既有列名 SELECT 不受影响；
--           新增 id 列对既有 SELECT 透明。外键 references agent_{prompts,tools,skills} 不变。

-- ===== agent_prompt_bindings =====
ALTER TABLE agent_prompt_bindings DROP CONSTRAINT IF EXISTS agent_prompt_bindings_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'agent_prompt_bindings'::regclass
      AND conname = 'agent_prompt_bindings_agent_prompt_key'
  ) THEN
    ALTER TABLE agent_prompt_bindings
      ADD CONSTRAINT agent_prompt_bindings_agent_prompt_key UNIQUE (agent_id, prompt_id);
  END IF;
END $$;

ALTER TABLE agent_prompt_bindings ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'agent_prompt_bindings'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE agent_prompt_bindings ADD PRIMARY KEY (id);
  END IF;
END $$;

-- ===== agent_tool_bindings =====
ALTER TABLE agent_tool_bindings DROP CONSTRAINT IF EXISTS agent_tool_bindings_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'agent_tool_bindings'::regclass
      AND conname = 'agent_tool_bindings_agent_tool_key'
  ) THEN
    ALTER TABLE agent_tool_bindings
      ADD CONSTRAINT agent_tool_bindings_agent_tool_key UNIQUE (agent_id, tool_id);
  END IF;
END $$;

ALTER TABLE agent_tool_bindings ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'agent_tool_bindings'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE agent_tool_bindings ADD PRIMARY KEY (id);
  END IF;
END $$;

-- ===== agent_skill_bindings =====
ALTER TABLE agent_skill_bindings DROP CONSTRAINT IF EXISTS agent_skill_bindings_pkey;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'agent_skill_bindings'::regclass
      AND conname = 'agent_skill_bindings_agent_skill_key'
  ) THEN
    ALTER TABLE agent_skill_bindings
      ADD CONSTRAINT agent_skill_bindings_agent_skill_key UNIQUE (agent_id, skill_id);
  END IF;
END $$;

ALTER TABLE agent_skill_bindings ADD COLUMN IF NOT EXISTS id bigint GENERATED ALWAYS AS IDENTITY;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'agent_skill_bindings'::regclass
      AND contype = 'p'
  ) THEN
    ALTER TABLE agent_skill_bindings ADD PRIMARY KEY (id);
  END IF;
END $$;
