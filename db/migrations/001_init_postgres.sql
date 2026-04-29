create sequence if not exists agents_im_users_id_seq;
create sequence if not exists agents_im_groups_id_seq;
create sequence if not exists agents_im_messages_id_seq;

create table if not exists users (
  user_id text primary key default ('usr_' || lpad(nextval('agents_im_users_id_seq')::text, 6, '0')),
  identifier text not null,
  display_name text not null,
  name text not null,
  gender text not null default 'unknown',
  age integer not null default 0,
  region text not null default '',
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint users_identifier_uniq unique (identifier),
  constraint users_identifier_not_blank check (identifier <> ''),
  constraint users_display_name_not_blank check (display_name <> ''),
  constraint users_name_not_blank check (name <> ''),
  constraint users_gender_check check (gender in ('unknown', 'male', 'female', 'other')),
  constraint users_age_check check (age >= 0 and age <= 150)
);

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
