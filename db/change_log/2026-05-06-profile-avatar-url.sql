-- Source of truth for Issue #4 avatar URL persistence.
-- Apply with db/migrations/007_profile_avatar_url.sql in normal migration order.

alter table if exists profiles
  add column if not exists avatar_url text not null default '';
