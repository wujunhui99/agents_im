-- agent_private_conversations 长期记忆 + 用户画像（#688）。
--
-- #686 的 rounds 从"硬砍 16 轮"改为"攒到 20 轮触发摘要"：摘要前 18 轮折进 long_term_memory、
-- 保留后 2 轮保证连续性；从这 18 轮抽取用户个人信息更新 user_profile。两者在组 prompt 时作为
-- 上下文注入 system 段。
--
--   · long_term_memory：滚动重摘要（旧记忆 + 18 轮 → 新记忆，可丢弃旧信息、精简），应用层限 ≤4096 字符。
--   · user_profile：用户画像（个人偏好/身份/长期事实），随每次摘要增量更新。
-- 均为 text，默认空串（存量行透明回填）。

ALTER TABLE agent_private_conversations
    ADD COLUMN IF NOT EXISTS long_term_memory text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS user_profile     text NOT NULL DEFAULT '';
