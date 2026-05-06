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
  trigger_server_msg_id text not null,
  trigger_event_id text not null default '',
  status text not null,
  response_server_msg_id text not null default '',
  error_message text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- Existing live databases may already have this table in a pre-contract shape.
-- CREATE TABLE IF NOT EXISTS does not add or rename columns, so normalize the
-- columns that later statements in this migration reference.
do $$
begin
  if exists (
    select 1 from information_schema.tables
    where table_schema = current_schema()
      and table_name = 'agent_trigger_idempotency'
  ) then
    if exists (
      select 1 from information_schema.columns
      where table_schema = current_schema()
        and table_name = 'agent_trigger_idempotency'
        and column_name = 'trigger_message_id'
    ) and not exists (
      select 1 from information_schema.columns
      where table_schema = current_schema()
        and table_name = 'agent_trigger_idempotency'
        and column_name = 'trigger_server_msg_id'
    ) then
      alter table agent_trigger_idempotency
        rename column trigger_message_id to trigger_server_msg_id;
    end if;

    if not exists (
      select 1 from information_schema.columns
      where table_schema = current_schema()
        and table_name = 'agent_trigger_idempotency'
        and column_name = 'trigger_server_msg_id'
    ) then
      alter table agent_trigger_idempotency
        add column trigger_server_msg_id text not null default '';
      alter table agent_trigger_idempotency
        alter column trigger_server_msg_id drop default;
    end if;

    if exists (
      select 1 from information_schema.columns
      where table_schema = current_schema()
        and table_name = 'agent_trigger_idempotency'
        and column_name = 'response_message_id'
    ) and not exists (
      select 1 from information_schema.columns
      where table_schema = current_schema()
        and table_name = 'agent_trigger_idempotency'
        and column_name = 'response_server_msg_id'
    ) then
      alter table agent_trigger_idempotency
        rename column response_message_id to response_server_msg_id;
    end if;

    if not exists (
      select 1 from information_schema.columns
      where table_schema = current_schema()
        and table_name = 'agent_trigger_idempotency'
        and column_name = 'response_server_msg_id'
    ) then
      alter table agent_trigger_idempotency
        add column response_server_msg_id text not null default '';
    end if;
  end if;
end $$;

create index if not exists agent_trigger_idempotency_trigger_idx
  on agent_trigger_idempotency (trigger_server_msg_id, agent_account_id);
