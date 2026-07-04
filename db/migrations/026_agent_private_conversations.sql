-- agent_private_conversations（#686）：agent 私聊 conversation store。agent 域自有、由 Kafka
-- 消费直写，作为 agent AI 应答上下文的**唯一源**（不再每次触发同步调 msg-rpc PullMessages 拉历史）。
--
-- 语义：
--   · 一会话一行（conversation_id 唯一），rounds(jsonb) 保存**最多 16 轮**（=16 次 agent 回复）
--     的 (user 批次, assistant 回复) 对，超出从最旧轮丢弃（滚动窗口）。
--   · pending(jsonb) 暂存尚未被 agent 消费的 user 消息：AI 正在回复时用户又追发的多条消息落这里，
--     一起在下一轮合并成一条 user 输入回复（合流 / coalescing），不逐条回复。
--   · state=running 期间的入站消息只累积到 pending；running 由 running_until 租约兜底崩溃回收。
--   · last_consumed_seq 做冪等：seq<=last_consumed_seq 的重复/回放事件直接丢弃。
--   · account 引用沿用 text 十进制串（account_id D16 先例，见 013 注释），不引 bigint::text keystone。
--
-- rounds 元素形状（应用层维护，DB 只当 jsonb 存）：
--   {"user":["...","..."],"assistant":"...","seq_from":<int>,"seq_to":<int>,"agent_run_id":"..."}
-- pending 元素形状：{"seq":<int>,"text":"...","server_msg_id":"...","created_at_ms":<int>}

CREATE TABLE IF NOT EXISTS agent_private_conversations (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    conversation_id    text        NOT NULL,
    agent_account_id   text        NOT NULL,
    peer_account_id    text        NOT NULL,
    rounds             jsonb       NOT NULL DEFAULT '[]'::jsonb,
    pending            jsonb       NOT NULL DEFAULT '[]'::jsonb,
    last_consumed_seq  bigint      NOT NULL DEFAULT 0,
    state              text        NOT NULL DEFAULT 'idle',
    running_until      timestamptz,
    version            bigint      NOT NULL DEFAULT 0,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT agent_private_conversations_conversation_id_key UNIQUE (conversation_id),
    CONSTRAINT agent_private_conversations_state_check CHECK (state IN ('idle', 'running'))
);

CREATE INDEX IF NOT EXISTS agent_private_conversations_agent_account_idx
    ON agent_private_conversations (agent_account_id);
