-- Convert DB-generated internal agent registry/audit IDs from prefixed text values
-- (for example prompt_000001) or numeric text values to bigint identity columns.
-- External/client IDs such as conversation_id, client_msg_id, trace_id and
-- account_id remain text because their wire format is string-based.

create or replace function agents_im_text_id_to_bigint(value text)
returns bigint
language sql
immutable
as $$
  select nullif(regexp_replace(value, '^.*?(\d+)$', '\1'), '')::bigint
$$;

alter table if exists agent_tools drop constraint if exists agent_tools_mcp_server_id_fkey;
alter table if exists agent_prompt_bindings drop constraint if exists agent_prompt_bindings_prompt_id_fkey;
alter table if exists agent_tool_bindings drop constraint if exists agent_tool_bindings_tool_id_fkey;
alter table if exists agent_skill_bindings drop constraint if exists agent_skill_bindings_skill_id_fkey;
alter table if exists agent_tool_calls drop constraint if exists agent_tool_calls_run_id_fkey;
alter table if exists agent_file_reads drop constraint if exists agent_file_reads_run_id_fkey;
alter table if exists agent_python_execs drop constraint if exists agent_python_execs_run_id_fkey;

alter table if exists agent_prompt_bindings drop constraint if exists agent_prompt_bindings_pkey;
alter table if exists agent_tool_bindings drop constraint if exists agent_tool_bindings_pkey;
alter table if exists agent_skill_bindings drop constraint if exists agent_skill_bindings_pkey;
alter table if exists agents drop constraint if exists agents_id_numeric_check;
alter table if exists agent_prompt_bindings drop constraint if exists agent_prompt_bindings_agent_not_blank;
alter table if exists agent_tool_bindings drop constraint if exists agent_tool_bindings_agent_not_blank;
alter table if exists agent_skill_bindings drop constraint if exists agent_skill_bindings_agent_not_blank;
alter table if exists agent_runs drop constraint if exists agent_runs_agent_id_not_blank;
alter table if exists agent_tool_calls drop constraint if exists agent_tool_calls_agent_id_not_blank;
alter table if exists agent_file_reads drop constraint if exists agent_file_reads_agent_id_not_blank;
alter table if exists agent_file_reads drop constraint if exists agent_file_reads_skill_id_not_blank;
alter table if exists agent_python_execs drop constraint if exists agent_python_execs_agent_id_not_blank;

alter table if exists agent_prompts alter column prompt_id drop identity if exists;
alter table if exists mcp_servers alter column server_id drop identity if exists;
alter table if exists agent_tools alter column tool_id drop identity if exists;
alter table if exists agent_skills alter column skill_id drop identity if exists;
alter table if exists agents alter column agent_id drop identity if exists;
alter table if exists agent_runs alter column run_id drop identity if exists;
alter table if exists agent_tool_calls alter column tool_call_id drop identity if exists;
alter table if exists agent_file_reads alter column file_read_id drop identity if exists;
alter table if exists agent_python_execs alter column python_exec_id drop identity if exists;

alter table if exists agent_prompts alter column prompt_id drop default;
alter table if exists mcp_servers alter column server_id drop default;
alter table if exists agent_tools alter column tool_id drop default;
alter table if exists agent_skills alter column skill_id drop default;
alter table if exists agent_runs alter column run_id drop default;
alter table if exists agent_tool_calls alter column tool_call_id drop default;
alter table if exists agent_file_reads alter column file_read_id drop default;
alter table if exists agent_python_execs alter column python_exec_id drop default;

alter table if exists agents alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_prompts alter column prompt_id type bigint using agents_im_text_id_to_bigint(prompt_id::text);
alter table if exists mcp_servers alter column server_id type bigint using agents_im_text_id_to_bigint(server_id::text);
alter table if exists agent_tools alter column mcp_server_id type bigint using agents_im_text_id_to_bigint(mcp_server_id::text);
alter table if exists agent_tools alter column tool_id type bigint using agents_im_text_id_to_bigint(tool_id::text);
alter table if exists agent_skills alter column skill_id type bigint using agents_im_text_id_to_bigint(skill_id::text);
alter table if exists agent_prompt_bindings alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_prompt_bindings alter column prompt_id type bigint using agents_im_text_id_to_bigint(prompt_id::text);
alter table if exists agent_tool_bindings alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_tool_bindings alter column tool_id type bigint using agents_im_text_id_to_bigint(tool_id::text);
alter table if exists agent_skill_bindings alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_skill_bindings alter column skill_id type bigint using agents_im_text_id_to_bigint(skill_id::text);
alter table if exists agent_runs alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_runs alter column run_id type bigint using agents_im_text_id_to_bigint(run_id::text);
alter table if exists agent_tool_calls alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_tool_calls alter column tool_id drop not null;
alter table if exists agent_tool_calls alter column tool_id drop default;
alter table if exists agent_tool_calls alter column tool_id type bigint using agents_im_text_id_to_bigint(tool_id::text);
alter table if exists agent_tool_calls alter column run_id type bigint using agents_im_text_id_to_bigint(run_id::text);
alter table if exists agent_tool_calls alter column tool_call_id type bigint using agents_im_text_id_to_bigint(tool_call_id::text);
alter table if exists agent_file_reads alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_file_reads alter column skill_id type bigint using agents_im_text_id_to_bigint(skill_id::text);
alter table if exists agent_file_reads alter column run_id type bigint using agents_im_text_id_to_bigint(run_id::text);
alter table if exists agent_file_reads alter column file_read_id type bigint using agents_im_text_id_to_bigint(file_read_id::text);
alter table if exists agent_python_execs alter column agent_id type bigint using agents_im_text_id_to_bigint(agent_id::text);
alter table if exists agent_python_execs alter column run_id type bigint using agents_im_text_id_to_bigint(run_id::text);
alter table if exists agent_python_execs alter column python_exec_id type bigint using agents_im_text_id_to_bigint(python_exec_id::text);

alter table if exists agent_prompts alter column prompt_id add generated by default as identity;
alter table if exists mcp_servers alter column server_id add generated by default as identity;
alter table if exists agent_tools alter column tool_id add generated by default as identity;
alter table if exists agent_skills alter column skill_id add generated by default as identity;
alter table if exists agents alter column agent_id add generated by default as identity;
alter table if exists agent_runs alter column run_id add generated by default as identity;
alter table if exists agent_tool_calls alter column tool_call_id add generated by default as identity;
alter table if exists agent_file_reads alter column file_read_id add generated by default as identity;
alter table if exists agent_python_execs alter column python_exec_id add generated by default as identity;

select setval(pg_get_serial_sequence('agent_prompts', 'prompt_id'), coalesce((select max(prompt_id) from agent_prompts), 0) + 1, false);
select setval(pg_get_serial_sequence('mcp_servers', 'server_id'), coalesce((select max(server_id) from mcp_servers), 0) + 1, false);
select setval(pg_get_serial_sequence('agent_tools', 'tool_id'), coalesce((select max(tool_id) from agent_tools), 0) + 1, false);
select setval(pg_get_serial_sequence('agent_skills', 'skill_id'), coalesce((select max(skill_id) from agent_skills), 0) + 1, false);
select setval(pg_get_serial_sequence('agents', 'agent_id'), coalesce((select max(agent_id) from agents), 0) + 1, false);
select setval(pg_get_serial_sequence('agent_runs', 'run_id'), coalesce((select max(run_id) from agent_runs), 0) + 1, false);
select setval(pg_get_serial_sequence('agent_tool_calls', 'tool_call_id'), coalesce((select max(tool_call_id) from agent_tool_calls), 0) + 1, false);
select setval(pg_get_serial_sequence('agent_file_reads', 'file_read_id'), coalesce((select max(file_read_id) from agent_file_reads), 0) + 1, false);
select setval(pg_get_serial_sequence('agent_python_execs', 'python_exec_id'), coalesce((select max(python_exec_id) from agent_python_execs), 0) + 1, false);

alter table if exists agent_prompt_bindings add primary key (agent_id, prompt_id);
alter table if exists agent_tool_bindings add primary key (agent_id, tool_id);
alter table if exists agent_skill_bindings add primary key (agent_id, skill_id);

alter table if exists agent_tools
  add constraint agent_tools_mcp_server_id_fkey foreign key (mcp_server_id) references mcp_servers(server_id) on delete restrict;
alter table if exists agent_prompt_bindings
  add constraint agent_prompt_bindings_prompt_id_fkey foreign key (prompt_id) references agent_prompts(prompt_id) on delete restrict;
alter table if exists agent_tool_bindings
  add constraint agent_tool_bindings_tool_id_fkey foreign key (tool_id) references agent_tools(tool_id) on delete restrict;
alter table if exists agent_skill_bindings
  add constraint agent_skill_bindings_skill_id_fkey foreign key (skill_id) references agent_skills(skill_id) on delete restrict;
alter table if exists agent_tool_calls
  add constraint agent_tool_calls_run_id_fkey foreign key (run_id) references agent_runs(run_id) on delete restrict;
alter table if exists agent_file_reads
  add constraint agent_file_reads_run_id_fkey foreign key (run_id) references agent_runs(run_id) on delete restrict;
alter table if exists agent_python_execs
  add constraint agent_python_execs_run_id_fkey foreign key (run_id) references agent_runs(run_id) on delete restrict;

drop function if exists agents_im_text_id_to_bigint(text);
