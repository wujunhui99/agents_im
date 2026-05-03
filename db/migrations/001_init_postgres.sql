create sequence if not exists agents_im_agent_prompts_id_seq;
create sequence if not exists agents_im_mcp_servers_id_seq;
create sequence if not exists agents_im_agent_tools_id_seq;
create sequence if not exists agents_im_agent_skills_id_seq;

create table if not exists accounts (
  account_id text primary key,
  identifier text not null,
  account_type smallint not null default 1,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint accounts_identifier_uniq unique (identifier)
);

create table if not exists profiles (
  account_id text primary key,
  display_name text not null,
  name text not null,
  gender smallint not null default 0,
  birth_date date,
  region text not null default '',
  avatar_media_id text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists media_objects (
  media_id text primary key,
  owner_account_id text not null default '',
  conversation_id text not null default '',
  storage_provider smallint not null default 1,
  bucket text not null,
  object_key text not null,
  original_filename text not null default '',
  content_type text not null,
  size_bytes bigint not null,
  purpose smallint not null,
  status smallint not null default 1,
  metadata jsonb not null default '{}'::jsonb,
  expires_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint media_objects_object_key_uniq unique (object_key)
);

create index if not exists media_objects_owner_status_created_idx
  on media_objects (owner_account_id, status, created_at desc);

create index if not exists media_objects_conversation_status_created_idx
  on media_objects (conversation_id, status, created_at desc);

create index if not exists media_objects_pending_expiry_idx
  on media_objects (status, expires_at)
  where status = 1 and expires_at is not null;

create table if not exists auth_credentials (
  account_id text primary key,
  password_hash text not null,
  password_algo smallint not null default 1,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists friendships (
  account_id text not null,
  friend_account_id text not null,
  status smallint not null default 1,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (account_id, friend_account_id)
);

create index if not exists friendships_account_status_idx
  on friendships (account_id, status, friend_account_id);

create index if not exists friendships_friend_account_idx
  on friendships (friend_account_id, account_id);

create table if not exists groups (
  group_id text primary key,
  name text not null,
  description text not null default '',
  creator_account_id text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists group_members (
  group_id text not null,
  account_id text not null,
  role smallint not null default 1,
  status smallint not null default 1,
  join_time timestamptz not null default now(),
  left_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (group_id, account_id)
);

create index if not exists group_members_group_status_idx
  on group_members (group_id, status, account_id);

create index if not exists group_members_account_status_idx
  on group_members (account_id, status, group_id);

create table if not exists conversation_threads (
  conversation_id text primary key,
  conversation_type smallint not null,
  single_account_a text not null default '',
  single_account_b text not null default '',
  group_id text not null default '',
  max_seq bigint not null default 0,
  last_message_id text not null default '',
  last_message_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create unique index if not exists conversation_threads_single_pair_uniq
  on conversation_threads (single_account_a, single_account_b)
  where conversation_type = 1;

create unique index if not exists conversation_threads_group_uniq
  on conversation_threads (group_id)
  where conversation_type = 2;

create table if not exists messages (
  message_id text primary key,
  client_msg_id text not null,
  sender_account_id text not null,
  conversation_id text not null,
  seq bigint not null,
  conversation_type smallint not null,
  receiver_account_id text not null default '',
  group_id text not null default '',
  content_type smallint not null,
  content jsonb not null,
  payload_hash text not null,
  client_send_time timestamptz,
  server_received_at timestamptz not null default now(),
  message_state smallint not null default 1,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  message_origin smallint not null default 1,
  agent_account_id text not null default '',
  trigger_message_id text not null default '',
  agent_run_id text not null default '',
  allow_recursive_trigger boolean not null default false,
  constraint messages_conversation_seq_uniq unique (conversation_id, seq),
  constraint messages_sender_client_msg_uniq unique (sender_account_id, client_msg_id)
);

create index if not exists messages_conversation_seq_idx
  on messages (conversation_id, seq);

create index if not exists messages_sender_created_idx
  on messages (sender_account_id, created_at desc);

create index if not exists messages_agent_run_idx
  on messages (agent_run_id)
  where agent_run_id <> '';

create table if not exists user_conversation_states (
  account_id text not null,
  conversation_id text not null,
  last_read_seq bigint not null default 0,
  visible_start_seq bigint not null default 0,
  muted boolean not null default false,
  archived boolean not null default false,
  pinned_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (account_id, conversation_id)
);

create index if not exists user_conversation_states_account_updated_idx
  on user_conversation_states (account_id, updated_at desc);

create index if not exists user_conversation_states_account_pinned_idx
  on user_conversation_states (account_id, pinned_at desc)
  where pinned_at is not null;

create table if not exists message_outbox (
  event_id text primary key,
  event_type smallint not null,
  aggregate_type smallint not null,
  aggregate_id text not null,
  conversation_id text not null,
  message_id text not null default '',
  seq bigint not null default 0,
  payload jsonb not null,
  status smallint not null default 1,
  attempt_count integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  locked_by text not null default '',
  locked_until timestamptz,
  last_error text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  published_at timestamptz,
  constraint message_outbox_event_aggregate_uniq unique (event_type, aggregate_type, aggregate_id)
);

create index if not exists message_outbox_pending_idx
  on message_outbox (status, next_attempt_at, created_at, event_id);

create index if not exists message_outbox_locked_idx
  on message_outbox (status, locked_until);

create index if not exists message_outbox_conversation_seq_idx
  on message_outbox (conversation_id, seq);

create index if not exists message_outbox_message_idx
  on message_outbox (message_id);

create table if not exists agent_prompts (
  prompt_id text primary key default ('prompt_' || lpad(nextval('agents_im_agent_prompts_id_seq')::text, 6, '0')),
  name text not null,
  description text not null default '',
  content text not null,
  variables_schema_json jsonb not null default '{}'::jsonb,
  version text not null,
  status text not null,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_prompts_name_not_blank check (name <> ''),
  constraint agent_prompts_content_not_blank check (content <> ''),
  constraint agent_prompts_version_not_blank check (version <> ''),
  constraint agent_prompts_created_by_not_blank check (created_by <> ''),
  constraint agent_prompts_status_check check (status in ('draft', 'active', 'archived')),
  constraint agent_prompts_name_version_uniq unique (name, version)
);

create index if not exists agent_prompts_status_updated_idx
  on agent_prompts (status, updated_at desc);

create table if not exists mcp_servers (
  server_id text primary key default ('mcp_srv_' || lpad(nextval('agents_im_mcp_servers_id_seq')::text, 6, '0')),
  name text not null,
  transport text not null,
  url text not null,
  config_json jsonb not null default '{}'::jsonb,
  headers_secret_ref text not null default '',
  timeout_seconds integer not null,
  status text not null,
  admin_configured boolean not null,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint mcp_servers_name_not_blank check (name <> ''),
  constraint mcp_servers_transport_check check (transport in ('http', 'sse', 'streamable_http')),
  constraint mcp_servers_url_not_blank check (url <> ''),
  constraint mcp_servers_timeout_check check (timeout_seconds > 0),
  constraint mcp_servers_status_check check (status in ('active', 'disabled', 'archived')),
  constraint mcp_servers_admin_configured_check check (admin_configured is true),
  constraint mcp_servers_created_by_not_blank check (created_by <> ''),
  constraint mcp_servers_name_uniq unique (name)
);

create index if not exists mcp_servers_status_updated_idx
  on mcp_servers (status, updated_at desc);

create table if not exists agent_tools (
  tool_id text primary key default ('tool_' || lpad(nextval('agents_im_agent_tools_id_seq')::text, 6, '0')),
  name text not null,
  description text not null default '',
  tool_type text not null,
  mcp_server_id text references mcp_servers(server_id) on delete restrict,
  mcp_tool_name text not null default '',
  local_handler_key text not null default '',
  builtin_key text not null default '',
  input_schema_json jsonb not null default '{}'::jsonb,
  output_schema_json jsonb not null default '{}'::jsonb,
  permission_level text not null default 'agent_bound',
  status text not null,
  admin_configured boolean not null,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_tools_name_not_blank check (name <> ''),
  constraint agent_tools_type_check check (tool_type in ('mcp', 'local', 'builtin')),
  constraint agent_tools_status_check check (status in ('active', 'disabled', 'archived')),
  constraint agent_tools_admin_configured_check check (admin_configured is true),
  constraint agent_tools_created_by_not_blank check (created_by <> ''),
  constraint agent_tools_permission_not_blank check (permission_level <> ''),
  constraint agent_tools_shape_check check (
    (tool_type = 'mcp' and mcp_server_id is not null and mcp_tool_name <> '' and local_handler_key = '' and builtin_key = '')
    or
    (tool_type = 'local' and mcp_server_id is null and mcp_tool_name = '' and local_handler_key <> '' and builtin_key = '')
    or
    (tool_type = 'builtin' and mcp_server_id is null and mcp_tool_name = '' and local_handler_key = '' and builtin_key <> '')
  ),
  constraint agent_tools_name_uniq unique (name)
);

create index if not exists agent_tools_status_type_idx
  on agent_tools (status, tool_type, updated_at desc);

create index if not exists agent_tools_mcp_server_idx
  on agent_tools (mcp_server_id)
  where mcp_server_id is not null;

create table if not exists agent_skills (
  skill_id text primary key default ('skill_' || lpad(nextval('agents_im_agent_skills_id_seq')::text, 6, '0')),
  name text not null,
  description text not null default '',
  version text not null,
  object_key text not null,
  sha256 text not null,
  content_type text not null,
  size_bytes bigint not null,
  status text not null,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agent_skills_name_not_blank check (name <> ''),
  constraint agent_skills_version_not_blank check (version <> ''),
  constraint agent_skills_object_key_not_blank check (object_key <> ''),
  constraint agent_skills_sha256_check check (sha256 ~ '^[0-9a-f]{64}$'),
  constraint agent_skills_content_type_not_blank check (content_type <> ''),
  constraint agent_skills_size_check check (size_bytes > 0),
  constraint agent_skills_status_check check (status in ('draft', 'active', 'archived')),
  constraint agent_skills_created_by_not_blank check (created_by <> ''),
  constraint agent_skills_name_version_uniq unique (name, version),
  constraint agent_skills_object_key_uniq unique (object_key)
);

create index if not exists agent_skills_status_updated_idx
  on agent_skills (status, updated_at desc);

create table if not exists agent_prompt_bindings (
  agent_id text not null,
  prompt_id text not null references agent_prompts(prompt_id) on delete restrict,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (agent_id, prompt_id),
  constraint agent_prompt_bindings_agent_not_blank check (agent_id <> ''),
  constraint agent_prompt_bindings_created_by_not_blank check (created_by <> '')
);

create index if not exists agent_prompt_bindings_prompt_idx
  on agent_prompt_bindings (prompt_id);

create table if not exists agent_tool_bindings (
  agent_id text not null,
  tool_id text not null references agent_tools(tool_id) on delete restrict,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (agent_id, tool_id),
  constraint agent_tool_bindings_agent_not_blank check (agent_id <> ''),
  constraint agent_tool_bindings_created_by_not_blank check (created_by <> '')
);

create index if not exists agent_tool_bindings_tool_idx
  on agent_tool_bindings (tool_id);

create table if not exists agent_skill_bindings (
  agent_id text not null,
  skill_id text not null references agent_skills(skill_id) on delete restrict,
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (agent_id, skill_id),
  constraint agent_skill_bindings_agent_not_blank check (agent_id <> ''),
  constraint agent_skill_bindings_created_by_not_blank check (created_by <> '')
);

create index if not exists agent_skill_bindings_skill_idx
  on agent_skill_bindings (skill_id);
