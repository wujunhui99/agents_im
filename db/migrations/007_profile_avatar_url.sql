-- Persist durable profile avatar display URLs.
-- The stored value is a stable application URL/reference, not image bytes,
-- object-storage credentials, raw private object keys, or presigned material.

alter table if exists profiles
  add column if not exists avatar_url text not null default '';
