create sequence if not exists agents_im_users_id_seq;
create sequence if not exists agents_im_groups_id_seq;
create sequence if not exists agents_im_messages_id_seq;
create sequence if not exists agents_im_outbox_events_id_seq;
create sequence if not exists agents_im_media_id_seq;
create sequence if not exists agents_im_agent_prompts_id_seq;
create sequence if not exists agents_im_mcp_servers_id_seq;
create sequence if not exists agents_im_agent_tools_id_seq;
create sequence if not exists agents_im_agent_skills_id_seq;

create table if not exists users (
  user_id text primary key default ('usr_' || lpad(nextval('agents_im_users_id_seq')::text, 6, '0')),
  identifier text not null,
  display_name text not null,
  name text not null,
  gender text not null default 'unknown',
  age integer not null default 0,
  region text not null default '',
  account_type text not null default 'user',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint users_identifier_uniq unique (identifier),
  constraint users_identifier_not_blank check (identifier <> ''),
  constraint users_display_name_not_blank check (display_name <> ''),
  constraint users_name_not_blank check (name <> ''),
  constraint users_gender_check check (gender in ('unknown', 'male', 'female', 'other')),
  constraint users_age_check check (age >= 0 and age <= 150),
  constraint users_account_type_check check (account_type in ('user', 'agent', 'admin'))
);

alter table users
  add column if not exists account_type text not null default 'user';

alter table users
  drop constraint if exists users_account_type_check;

update users
set account_type = 'user'
where account_type = 'normal';

alter table users
  alter column account_type set default 'user';

alter table users
  add constraint users_account_type_check check (account_type in ('user', 'agent', 'admin'));

alter table users
  add column if not exists avatar_media_id text not null default '';

create table if not exists media_objects (
  media_id text primary key default ('med_' || lpad(nextval('agents_im_media_id_seq')::text, 6, '0')),
  owner_user_id text not null references users(user_id) on delete restrict,
  bucket text not null,
  object_key text not null,
  sha256 text not null default '',
  content_type text not null,
  size_bytes bigint not null,
  width integer,
  height integer,
  original_filename text not null default '',
  purpose text not null,
  status text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint media_objects_owner_not_blank check (owner_user_id <> ''),
  constraint media_objects_bucket_not_blank check (bucket <> ''),
  constraint media_objects_object_key_not_blank check (object_key <> ''),
  constraint media_objects_object_key_uniq unique (object_key),
  constraint media_objects_sha256_check check (sha256 = '' or sha256 ~ '^[0-9a-f]{64}$'),
  constraint media_objects_content_type_not_blank check (content_type <> ''),
  constraint media_objects_size_check check (size_bytes > 0),
  constraint media_objects_dimensions_check check (
    (width is null or width > 0)
    and
    (height is null or height > 0)
  ),
  constraint media_objects_purpose_check check (purpose in ('avatar', 'message_image', 'message_file', 'agent_skill')),
  constraint media_objects_status_check check (status in ('pending', 'ready', 'rejected', 'deleted'))
);

create index if not exists media_objects_owner_status_created_idx
  on media_objects (owner_user_id, status, created_at desc);

create index if not exists media_objects_sha256_idx
  on media_objects (sha256)
  where sha256 <> '';

create table if not exists auth_credentials (
  identifier text primary key,
  user_id text not null,
  password_hash text not null,
  salt text not null,
  hash_version text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint auth_credentials_user_id_uniq unique (user_id),
  constraint auth_credentials_identifier_not_blank check (identifier <> ''),
  constraint auth_credentials_user_id_not_blank check (user_id <> ''),
  constraint auth_credentials_password_hash_not_blank check (password_hash <> ''),
  constraint auth_credentials_hash_version_not_blank check (hash_version <> '')
);

create table if not exists friendships (
  user_id text not null,
  friend_id text not null,
  status text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (user_id, friend_id),
  constraint friendships_distinct_users_check check (user_id <> friend_id),
  constraint friendships_status_check check (status in ('active', 'deleted'))
);

create index if not exists friendships_user_status_idx
  on friendships (user_id, status, friend_id);

create table if not exists groups (
  group_id text primary key default ('grp_' || lpad(nextval('agents_im_groups_id_seq')::text, 6, '0')),
  name text not null,
  description text not null default '',
  creator_user_id text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint groups_name_not_blank check (name <> ''),
  constraint groups_creator_not_blank check (creator_user_id <> '')
);

create table if not exists group_members (
  group_id text not null references groups(group_id) on delete cascade,
  user_id text not null,
  state text not null,
  joined_at timestamptz not null default now(),
  left_at timestamptz,
  primary key (group_id, user_id),
  constraint group_members_user_not_blank check (user_id <> ''),
  constraint group_members_state_check check (state in ('active', 'left')),
  constraint group_members_left_at_check check (
    (state = 'active' and left_at is null)
    or
    (state = 'left' and left_at is not null)
  )
);

create index if not exists group_members_group_state_idx
  on group_members (group_id, state, user_id);

create table if not exists conversation_threads (
  conversation_id text primary key,
  chat_type text not null,
  single_user_a text not null default '',
  single_user_b text not null default '',
  group_id text not null default '',
  max_seq bigint not null default 0,
  last_message_id text not null default '',
  last_message_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint conversation_threads_chat_type_check check (chat_type in ('single', 'group')),
  constraint conversation_threads_max_seq_check check (max_seq >= 0),
  constraint conversation_threads_shape_check check (
    (chat_type = 'single' and single_user_a <> '' and single_user_b <> '' and single_user_a < single_user_b and group_id = '')
    or
    (chat_type = 'group' and group_id <> '' and single_user_a = '' and single_user_b = '')
  )
);

create unique index if not exists conversation_threads_single_pair_uniq
  on conversation_threads (single_user_a, single_user_b)
  where chat_type = 'single';

create unique index if not exists conversation_threads_group_uniq
  on conversation_threads (group_id)
  where chat_type = 'group';

create table if not exists messages (
  server_msg_id text primary key default ('msg_' || lpad(nextval('agents_im_messages_id_seq')::text, 6, '0')),
  client_msg_id text not null,
  sender_id text not null,
  conversation_id text not null references conversation_threads(conversation_id) on delete restrict,
  seq bigint not null,
  chat_type text not null,
  receiver_id text not null default '',
  group_id text not null default '',
  content_type text not null,
  content jsonb not null,
  payload_hash text not null,
  send_time timestamptz not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint messages_conversation_seq_uniq unique (conversation_id, seq),
  constraint messages_sender_client_msg_uniq unique (sender_id, client_msg_id),
  constraint messages_chat_type_check check (chat_type in ('single', 'group')),
  constraint messages_seq_positive_check check (seq > 0),
  constraint messages_content_type_check check (content_type <> ''),
  constraint messages_content_shape_check check (
    (chat_type = 'single' and receiver_id <> '' and group_id = '')
    or
    (chat_type = 'group' and group_id <> '' and receiver_id = '')
  )
);

create index if not exists messages_conversation_seq_idx
  on messages (conversation_id, seq);

create index if not exists messages_sender_created_idx
  on messages (sender_id, created_at desc);

create table if not exists user_conversation_states (
  user_id text not null,
  conversation_id text not null references conversation_threads(conversation_id) on delete cascade,
  has_read_seq bigint not null default 0,
  last_visible_seq bigint not null default 0,
  muted boolean not null default false,
  archived boolean not null default false,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (user_id, conversation_id),
  constraint user_conversation_states_read_seq_check check (has_read_seq >= 0),
  constraint user_conversation_states_visible_seq_check check (last_visible_seq >= 0),
  constraint user_conversation_states_read_visible_check check (has_read_seq <= last_visible_seq)
);

create index if not exists user_conversation_states_user_updated_idx
  on user_conversation_states (user_id, updated_at desc);

create table if not exists message_idempotency_keys (
  sender_id text not null,
  client_msg_id text not null,
  payload_hash text not null,
  server_msg_id text not null references messages(server_msg_id) on delete restrict,
  conversation_id text not null,
  seq bigint not null,
  status text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (sender_id, client_msg_id),
  constraint message_idempotency_status_check check (status in ('accepted')),
  constraint message_idempotency_seq_positive_check check (seq > 0),
  constraint message_idempotency_server_msg_uniq unique (server_msg_id)
);

create index if not exists message_idempotency_conversation_seq_idx
  on message_idempotency_keys (conversation_id, seq);

create table if not exists message_outbox (
  event_id text primary key default ('outbox_' || lpad(nextval('agents_im_outbox_events_id_seq')::text, 6, '0')),
  event_type text not null,
  aggregate_type text not null,
  aggregate_id text not null,
  conversation_id text not null,
  server_msg_id text not null references messages(server_msg_id) on delete restrict,
  seq bigint not null,
  payload jsonb not null,
  status text not null default 'pending',
  attempt_count integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  locked_by text not null default '',
  locked_until timestamptz,
  last_error text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  published_at timestamptz,
  constraint message_outbox_event_type_not_blank check (event_type <> ''),
  constraint message_outbox_aggregate_type_not_blank check (aggregate_type <> ''),
  constraint message_outbox_aggregate_id_not_blank check (aggregate_id <> ''),
  constraint message_outbox_conversation_id_not_blank check (conversation_id <> ''),
  constraint message_outbox_seq_positive_check check (seq > 0),
  constraint message_outbox_status_check check (status in ('pending', 'published', 'failed')),
  constraint message_outbox_attempt_count_check check (attempt_count >= 0),
  constraint message_outbox_lock_shape_check check (
    (locked_by = '' and locked_until is null)
    or
    (locked_by <> '' and locked_until is not null)
  ),
  constraint message_outbox_published_shape_check check (
    (status = 'published' and published_at is not null)
    or
    (status <> 'published')
  ),
  constraint message_outbox_event_aggregate_uniq unique (event_type, aggregate_type, aggregate_id)
);

create index if not exists message_outbox_pending_idx
  on message_outbox (next_attempt_at, created_at, event_id)
  where status = 'pending';

create index if not exists message_outbox_locked_idx
  on message_outbox (locked_until)
  where status = 'pending' and locked_until is not null;

create index if not exists message_outbox_conversation_seq_idx
  on message_outbox (conversation_id, seq);

create index if not exists message_outbox_server_msg_idx
  on message_outbox (server_msg_id);

create table if not exists delivery_attempts (
  server_msg_id text not null references messages(server_msg_id) on delete cascade,
  conversation_id text not null,
  recipient_user_id text not null,
  status text not null,
  attempt_count integer not null default 0,
  last_error text not null default '',
  next_retry_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  primary key (server_msg_id, recipient_user_id),
  constraint delivery_attempts_server_msg_id_not_blank check (server_msg_id <> ''),
  constraint delivery_attempts_conversation_id_not_blank check (conversation_id <> ''),
  constraint delivery_attempts_recipient_user_id_not_blank check (recipient_user_id <> ''),
  constraint delivery_attempts_status_check check (status in ('accepted', 'published', 'delivered', 'offline', 'failed')),
  constraint delivery_attempts_attempt_count_check check (attempt_count >= 0),
  constraint delivery_attempts_retry_shape_check check (
    (status = 'failed')
    or
    (next_retry_at is null)
  )
);

create index if not exists delivery_attempts_conversation_recipient_idx
  on delivery_attempts (conversation_id, recipient_user_id, updated_at desc);

create index if not exists delivery_attempts_retry_idx
  on delivery_attempts (next_retry_at, updated_at)
  where status = 'failed' and next_retry_at is not null;

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
