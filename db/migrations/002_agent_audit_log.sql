create sequence if not exists agents_im_agent_runs_id_seq;
create sequence if not exists agents_im_agent_tool_calls_id_seq;
create sequence if not exists agents_im_agent_file_reads_id_seq;
create sequence if not exists agents_im_agent_python_execs_id_seq;

create table if not exists agent_runs (
  run_id text primary key default ('run_' || lpad(nextval('agents_im_agent_runs_id_seq')::text, 6, '0')),
  agent_id text not null,
  conversation_id text not null default '',
  trigger_message_id text not null default '',
  requesting_user_id text not null default '',
  status text not null,
  input_summary jsonb not null default '{}'::jsonb,
  output_summary jsonb not null default '{}'::jsonb,
  output_message_id text not null default '',
  error_code text not null default '',
  error_message text not null default '',
  trace_id text not null default '',
  request_id text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  created_at timestamptz not null default now(),
  constraint agent_runs_agent_id_not_blank check (agent_id <> ''),
  constraint agent_runs_status_check check (status in ('started', 'succeeded', 'failed', 'cancelled')),
  constraint agent_runs_failed_error_check check (
    status <> 'failed' or error_code <> '' or error_message <> ''
  ),
  constraint agent_runs_finished_after_started_check check (
    finished_at is null or finished_at >= started_at
  )
);

create index if not exists agent_runs_agent_created_idx
  on agent_runs (agent_id, created_at desc, run_id);

create index if not exists agent_runs_conversation_created_idx
  on agent_runs (conversation_id, created_at desc, run_id)
  where conversation_id <> '';

create table if not exists agent_tool_calls (
  tool_call_id text primary key default ('tool_call_' || lpad(nextval('agents_im_agent_tool_calls_id_seq')::text, 6, '0')),
  run_id text not null references agent_runs(run_id) on delete restrict,
  agent_id text not null,
  tool_id text not null default '',
  tool_name text not null,
  status text not null,
  input_summary jsonb not null default '{}'::jsonb,
  output_summary jsonb not null default '{}'::jsonb,
  duration_ms bigint not null default 0,
  error_code text not null default '',
  error_message text not null default '',
  trace_id text not null default '',
  request_id text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  created_at timestamptz not null default now(),
  constraint agent_tool_calls_agent_id_not_blank check (agent_id <> ''),
  constraint agent_tool_calls_tool_name_not_blank check (tool_name <> ''),
  constraint agent_tool_calls_status_check check (status in ('started', 'succeeded', 'failed', 'cancelled')),
  constraint agent_tool_calls_duration_check check (duration_ms >= 0),
  constraint agent_tool_calls_failed_error_check check (
    status <> 'failed' or error_code <> '' or error_message <> ''
  ),
  constraint agent_tool_calls_finished_after_started_check check (
    finished_at is null or finished_at >= started_at
  )
);

create index if not exists agent_tool_calls_run_created_idx
  on agent_tool_calls (run_id, created_at asc, tool_call_id);

create table if not exists agent_file_reads (
  file_read_id text primary key default ('file_read_' || lpad(nextval('agents_im_agent_file_reads_id_seq')::text, 6, '0')),
  run_id text not null references agent_runs(run_id) on delete restrict,
  agent_id text not null,
  skill_id text not null,
  file_id text not null default '',
  object_key text not null,
  sha256 text not null default '',
  status text not null,
  byte_count bigint not null default 0,
  content_summary jsonb not null default '{}'::jsonb,
  error_code text not null default '',
  error_message text not null default '',
  trace_id text not null default '',
  request_id text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  created_at timestamptz not null default now(),
  constraint agent_file_reads_agent_id_not_blank check (agent_id <> ''),
  constraint agent_file_reads_skill_id_not_blank check (skill_id <> ''),
  constraint agent_file_reads_object_key_not_blank check (object_key <> ''),
  constraint agent_file_reads_status_check check (status in ('started', 'succeeded', 'failed', 'cancelled')),
  constraint agent_file_reads_byte_count_check check (byte_count >= 0),
  constraint agent_file_reads_failed_error_check check (
    status <> 'failed' or error_code <> '' or error_message <> ''
  ),
  constraint agent_file_reads_finished_after_started_check check (
    finished_at is null or finished_at >= started_at
  )
);

create index if not exists agent_file_reads_run_created_idx
  on agent_file_reads (run_id, created_at asc, file_read_id);

create table if not exists agent_python_execs (
  python_exec_id text primary key default ('python_exec_' || lpad(nextval('agents_im_agent_python_execs_id_seq')::text, 6, '0')),
  run_id text not null references agent_runs(run_id) on delete restrict,
  agent_id text not null,
  sandbox_request_id text not null default '',
  status text not null,
  code_summary jsonb not null default '{}'::jsonb,
  resource_summary jsonb not null default '{}'::jsonb,
  stdout_summary jsonb not null default '{}'::jsonb,
  stderr_summary jsonb not null default '{}'::jsonb,
  result_summary jsonb not null default '{}'::jsonb,
  error_code text not null default '',
  error_message text not null default '',
  trace_id text not null default '',
  request_id text not null default '',
  started_at timestamptz not null default now(),
  finished_at timestamptz,
  created_at timestamptz not null default now(),
  constraint agent_python_execs_agent_id_not_blank check (agent_id <> ''),
  constraint agent_python_execs_status_check check (status in ('started', 'succeeded', 'failed', 'cancelled')),
  constraint agent_python_execs_failed_error_check check (
    status <> 'failed' or error_code <> '' or error_message <> ''
  ),
  constraint agent_python_execs_finished_after_started_check check (
    finished_at is null or finished_at >= started_at
  )
);

create index if not exists agent_python_execs_run_created_idx
  on agent_python_execs (run_id, created_at asc, python_exec_id);

create or replace function reject_agent_audit_mutation()
returns trigger
language plpgsql
as $$
begin
  raise exception 'agent audit records are append-only';
end;
$$;

do $$
begin
  if not exists (select 1 from pg_trigger where tgname = 'agent_runs_append_only_trg') then
    create trigger agent_runs_append_only_trg
    before update or delete on agent_runs
    for each row execute function reject_agent_audit_mutation();
  end if;

  if not exists (select 1 from pg_trigger where tgname = 'agent_tool_calls_append_only_trg') then
    create trigger agent_tool_calls_append_only_trg
    before update or delete on agent_tool_calls
    for each row execute function reject_agent_audit_mutation();
  end if;

  if not exists (select 1 from pg_trigger where tgname = 'agent_file_reads_append_only_trg') then
    create trigger agent_file_reads_append_only_trg
    before update or delete on agent_file_reads
    for each row execute function reject_agent_audit_mutation();
  end if;

  if not exists (select 1 from pg_trigger where tgname = 'agent_python_execs_append_only_trg') then
    create trigger agent_python_execs_append_only_trg
    before update or delete on agent_python_execs
    for each row execute function reject_agent_audit_mutation();
  end if;
end $$;
