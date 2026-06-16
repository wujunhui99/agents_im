-- EPIC #527 §1+§2 / issue #541: media_objects 雪花 bigint 主键 + uploader_id + 去 conversation_id
-- + 去 object_key 唯一约束 + digest_algo + 保留期 3 列。媒体存储重做的 schema 地基。
--
-- 迁移策略(ADR media-msg-id-bigint-migration #529 的例外):生产仅 2 条头像存量,走 #546
-- 就地迁移(下载→删→改列→回灌雪花 id),不清表。故本文件不 truncate;media_id 改 bigint 时
-- 表已为空(CI 全新库本就空;生产 #546 在 alter 前已删 2 行),using media_id::bigint 不对任何行求值。
-- 新 media_id 由 media-rpc 的 RoutedFlake(#528,HintBits=0)发,wire 仍十进制字符串(ADR #529)。
--
-- 列变更(§1/§2):
--   · media_id text→bigint:主键改实际整型(PK 索引局部性)。
--   · owner_account_id → uploader_id:文件是内容、不绑所有权语义,记上传者。account 引用
--     沿用 text 十进制串(account_id D16 先例,见 013 注释),本列不改类型。
--   · 删 conversation_id 及其索引:一份文件可被转发到多个会话,不应绑单一会话。
--   · 删 media_objects_object_key_uniq:文件级去重下多 media 行共享同 object_key(整文件 sha256)。
--   · 加 digest_algo smallint:标 object_key 承载的哈希算法(0=未指定,1=SHA256);默认 0,
--     待 #546 OSS 迁移把 object_key 落成 agents_im/{sha256} 后置 1(在此之前不谎称 SHA256)。
--   · 加保留期 3 列 last_used_at / expire_time / last_used_by(account_id bigint,对齐 §0 单级 GC)。
--   · 不再有 blocklist / 块表(不分片)。

drop index if exists media_objects_conversation_status_created_idx;
drop index if exists media_objects_owner_status_created_idx;

alter table media_objects drop column if exists conversation_id;
alter table media_objects rename column owner_account_id to uploader_id;
alter table media_objects alter column media_id type bigint using media_id::bigint;
alter table media_objects drop constraint if exists media_objects_object_key_uniq;

alter table media_objects add column if not exists digest_algo smallint not null default 0;
alter table media_objects add column if not exists last_used_at timestamptz;
alter table media_objects add column if not exists expire_time timestamptz;
alter table media_objects add column if not exists last_used_by bigint;

create index if not exists media_objects_uploader_status_created_idx
  on media_objects (uploader_id, status, created_at desc);
