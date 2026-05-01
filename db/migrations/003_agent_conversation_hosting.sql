alter table messages
  add column if not exists message_origin text not null default 'human',
  add column if not exists agent_account_id text not null default '',
  add column if not exists trigger_server_msg_id text not null default '',
  add column if not exists agent_run_id text not null default '',
  add column if not exists allow_recursive_trigger boolean not null default false;

alter table messages
  drop constraint if exists messages_message_origin_check,
  drop constraint if exists messages_agent_metadata_shape_check;

alter table messages
  add constraint messages_message_origin_check check (message_origin in ('human', 'ai', 'system')),
  add constraint messages_agent_metadata_shape_check check (
    (
      message_origin = 'ai'
      and agent_account_id <> ''
      and agent_account_id = sender_id
    )
    or
    (
      message_origin in ('human', 'system')
      and agent_account_id = ''
      and trigger_server_msg_id = ''
      and agent_run_id = ''
      and allow_recursive_trigger is false
    )
  );

create table if not exists agent_conversation_hosting (
  conversation_id text primary key,
  agent_account_id text not null,
  enabled boolean not null default true,
  allow_agent_message_recursion boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_conversation_hosting_conversation_not_blank check (conversation_id <> ''),
  constraint agent_conversation_hosting_agent_not_blank check (agent_account_id <> '')
);

create index if not exists agent_conversation_hosting_agent_enabled_idx
  on agent_conversation_hosting (agent_account_id, enabled, updated_at desc);

create table if not exists agent_trigger_idempotency (
  idempotency_key text primary key,
  conversation_id text not null,
  agent_account_id text not null,
  trigger_server_msg_id text not null,
  trigger_event_id text not null default '',
  status text not null,
  response_server_msg_id text not null default '',
  error_message text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_trigger_idempotency_key_not_blank check (idempotency_key <> ''),
  constraint agent_trigger_idempotency_conversation_not_blank check (conversation_id <> ''),
  constraint agent_trigger_idempotency_agent_not_blank check (agent_account_id <> ''),
  constraint agent_trigger_idempotency_trigger_not_blank check (trigger_server_msg_id <> ''),
  constraint agent_trigger_idempotency_status_check check (status in ('running', 'succeeded', 'failed'))
);

create index if not exists agent_trigger_idempotency_trigger_idx
  on agent_trigger_idempotency (trigger_server_msg_id, agent_account_id);
