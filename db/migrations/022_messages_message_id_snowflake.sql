-- EPIC #527 §0 / issue #531: messages.message_id text→雪花 bigint。
-- 与 media_id / account_id 同属项目雪花方案（RoutedFlake，pkg/idgen/routedflake.go）：
--   1 符号 + 41 时间戳(ms since 2026-01-01) + 12 位中段 + 10 序列号 = 64。
--   msg HintBits=1：中段最高位（bit 21）单聊=1/群聊=0（100… vs 000…）；机器号靠右（低 10 位，
--   bit 10..19），bit 20 预留。新 message_id 运行期由 msg-rpc 的 RoutedFlake 发，wire 仍十进制字符串(ADR #529)。
--
-- 迁移策略(ADR #529)：messages 走 clean cutover，生产切换时表已空，本文件的 UPDATE/CTE 对 0 行求值；
-- CI 全新库本就空。但为兼容「存量需就地迁移」的环境，本文件不 truncate：对每行按 created_at 重算雪花 id
-- （中段=0；同毫秒用 row_number 占序列号），并同步重写自引用 trigger_message_id 与
-- conversation_threads.last_message_id，最后再 alter 列类型。重算后若两行撞同一 id（同毫秒 > 1024 条），
-- 主键唯一约束会让迁移显式失败（失败优先，不静默覆盖）。

-- 1) 旧 text id → 新 bigint id 映射：按 created_at 构造雪花（41 时间戳 + 中段0 + 10 序列号）。
create temporary table _msg_id_map on commit drop as
with ticks as (
  select
    message_id as old_id,
    conversation_type,
    created_at,
    greatest((extract(epoch from created_at) * 1000)::bigint - 1767225600000, 0) as tick
  from messages
)
select
  old_id,
  (
    (tick << 22)
    | ((case when conversation_type = 1 then 1::bigint else 0::bigint end) << 21)
    | ((row_number() over (partition by tick order by created_at, old_id) - 1) & 1023)
  ) as new_id
from ticks;

-- 2) 先改自引用列（仍为 text，写新十进制串），再改主键值，保证引用一致。
update messages m
set trigger_message_id = mm.new_id::text
from _msg_id_map mm
where m.trigger_message_id = mm.old_id;

update conversation_threads ct
set last_message_id = mm.new_id::text
from _msg_id_map mm
where ct.last_message_id = mm.old_id;

update messages m
set message_id = mm.new_id::text
from _msg_id_map mm
where m.message_id = mm.old_id;

-- 3) 列类型 text→bigint：此时所有值均为十进制串（空表则无行求值）。
alter table messages alter column message_id type bigint using message_id::bigint;
