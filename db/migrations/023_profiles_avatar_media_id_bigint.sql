-- issue #550 / ADR media-msg-id-bigint-migration #529 约束 #1:profiles.avatar_media_id text→bigint。
-- #527/#541 已把 media_objects.media_id 改雪花 bigint(上线),#546 把线上头像引用回填为十进制串,
-- 本迁移把 user 域的头像引用列也对齐成 bigint,消除 text-vs-bigint 不一致。
--
-- 迁移策略:
--   · 现网 avatar_media_id 取值要么是十进制雪花串(#546 回填),要么是空账号的 '' 哨兵。
--     '' 无法 ::bigint,先把空/空白串归一成 '0';0 = 无头像哨兵(雪花 id 恒正,0 不冲突)。
--   · 列原为 text not null default '';改类型前先 drop default(text 默认无法承接 bigint),
--     using avatar_media_id::bigint 就地转型,再把 default 设回 0。not null 由 alter type 保留。
--   · wire/proto 仍是十进制字符串(ADR §2);int64↔string 转换在 user-rpc 边界做(0↔"")。

update profiles set avatar_media_id = '0' where btrim(avatar_media_id) = '';

alter table profiles alter column avatar_media_id drop default;
alter table profiles alter column avatar_media_id type bigint using avatar_media_id::bigint;
alter table profiles alter column avatar_media_id set default 0;
