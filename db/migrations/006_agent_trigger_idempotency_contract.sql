-- Align agent trigger idempotency table with repository contract.

do $$
begin
  if exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'trigger_message_id'
  ) and not exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'trigger_server_msg_id'
  ) then
    alter table agent_trigger_idempotency
      rename column trigger_message_id to trigger_server_msg_id;
  end if;

  if exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'response_message_id'
  ) and not exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'response_server_msg_id'
  ) then
    alter table agent_trigger_idempotency
      rename column response_message_id to response_server_msg_id;
  end if;

  if exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'status'
      and data_type <> 'text'
  ) then
    alter table agent_trigger_idempotency
      alter column status type text using
        case status::text
          when '1' then 'running'
          when '2' then 'succeeded'
          when '3' then 'failed'
          else status::text
        end;
  end if;
end $$;

drop index if exists agent_trigger_idempotency_trigger_idx;
create index if not exists agent_trigger_idempotency_trigger_idx
  on agent_trigger_idempotency (trigger_server_msg_id, agent_account_id);
