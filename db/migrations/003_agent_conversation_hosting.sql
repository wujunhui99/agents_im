-- Agent conversation hosting metadata.
-- Message agent columns are created by 001_init_postgres.sql in schema V2.

create table if not exists agent_conversation_hosting (
  conversation_id text primary key,
  agent_account_id text not null,
  enabled boolean not null default true,
  allow_agent_message_recursion boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists agent_conversation_hosting_agent_enabled_idx
  on agent_conversation_hosting (agent_account_id, enabled, updated_at desc);

create table if not exists agent_trigger_idempotency (
  idempotency_key text primary key,
  conversation_id text not null,
  agent_account_id text not null,
  trigger_message_id text not null,
  trigger_event_id text not null default '',
  status smallint not null,
  response_message_id text not null default '',
  error_message text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists agent_trigger_idempotency_trigger_idx
  on agent_trigger_idempotency (trigger_message_id, agent_account_id);
