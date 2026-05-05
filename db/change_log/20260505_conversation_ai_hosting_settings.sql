-- Source of truth for Issue #3 AI Conversation Hosting V1 schema.
-- Apply with db/migrations/005_conversation_ai_hosting_settings.sql in normal migration order.

create table if not exists conversation_ai_hosting_settings (
  owner_account_id text not null,
  conversation_id text not null,
  enabled boolean not null default false,
  mode text not null default 'auto_reply',
  max_recent_messages integer not null default 30,
  summary_enabled boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (owner_account_id, conversation_id),
  constraint conversation_ai_hosting_conversation_direct_check check (conversation_id like 'single:%'),
  constraint conversation_ai_hosting_owner_check check (owner_account_id <> '' and position(':' in owner_account_id) = 0),
  constraint conversation_ai_hosting_mode_check check (mode in ('auto_reply')),
  constraint conversation_ai_hosting_recent_check check (max_recent_messages between 1 and 30)
);

create unique index if not exists conversation_ai_hosting_one_enabled_owner_idx
  on conversation_ai_hosting_settings (conversation_id)
  where enabled = true;

create index if not exists conversation_ai_hosting_owner_updated_idx
  on conversation_ai_hosting_settings (owner_account_id, updated_at desc);
