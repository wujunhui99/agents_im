-- Fixture for upgrading old live databases where agent_trigger_idempotency
-- already exists with the pre-contract trigger_message_id column. This models
-- the deployment failure where 003 was replayed and attempted to index the
-- newer trigger_server_msg_id column before the old table shape was repaired.

create table if not exists schema_migrations (
  version text primary key,
  checksum text not null,
  applied_at timestamptz not null default now()
);

create table accounts (
  account_id text primary key,
  identifier text not null unique,
  display_name text not null,
  account_type text not null default 'user',
  status text not null default 'active',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table agent_trigger_idempotency (
  idempotency_key text primary key,
  conversation_id text not null,
  agent_account_id text not null,
  trigger_message_id text not null,
  trigger_event_id text not null default '',
  status integer not null,
  response_message_id text not null default '',
  error_message text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

insert into agent_trigger_idempotency (
  idempotency_key,
  conversation_id,
  agent_account_id,
  trigger_message_id,
  status
) values (
  'fixture-key',
  'conv_fixture',
  'agent_fixture',
  'msg_fixture',
  2
);

-- Mark early migrations as applied to model an existing live DB and force the
-- migrator to exercise the compatibility migration instead of replaying 003.
insert into schema_migrations (version, checksum) values
  ('001_init_postgres.sql', 'legacy-adopted-001'),
  ('002_agent_audit_log.sql', 'legacy-adopted-002'),
  ('002_agent_management.sql', 'legacy-adopted-002-management'),
  ('003_agent_conversation_hosting.sql', 'legacy-adopted-003'),
  ('004_backfill_direct_conversation_states.sql', 'legacy-adopted-004'),
  ('005_conversation_ai_hosting_settings.sql', 'legacy-adopted-005');
