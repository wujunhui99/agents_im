create sequence if not exists agents_im_agents_id_seq;

create table if not exists agents (
  agent_id text primary key default ('agt_' || lpad(nextval('agents_im_agents_id_seq')::text, 6, '0')),
  im_user_id text not null references users(user_id) on delete restrict,
  name text not null,
  description text not null default '',
  status text not null default 'disabled',
  created_by text not null,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint agents_im_user_id_uniq unique (im_user_id),
  constraint agents_im_user_id_not_blank check (im_user_id <> ''),
  constraint agents_name_not_blank check (name <> ''),
  constraint agents_created_by_not_blank check (created_by <> ''),
  constraint agents_status_check check (status in ('draft', 'active', 'disabled', 'archived'))
);

create index if not exists agents_status_created_idx
  on agents (status, created_at, agent_id);

create index if not exists agents_created_by_idx
  on agents (created_by, created_at, agent_id);
