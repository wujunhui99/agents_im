-- Issue #77: canonical default assistant friend/contact.
-- Creates or migrates agent_creator and accepted friendships for human users.

update accounts
set identifier = 'agent_creator',
    account_type = 2,
    updated_at = now()
where identifier = 'agent_father'
  and not exists (select 1 from accounts where identifier = 'agent_creator');

update accounts
set identifier = 'agent_creator_legacy_' || account_id,
    account_type = 2,
    updated_at = now()
where identifier = 'agent_father'
  and exists (select 1 from accounts where identifier = 'agent_creator');

do $$
begin
  if exists (
    select 1
    from information_schema.columns
    where table_name = 'accounts'
      and column_name = 'display_name'
  ) then
    insert into accounts (account_id, identifier, account_type, display_name)
    select '900000000000000077', 'agent_creator', 2, 'AI 助手'
    where not exists (select 1 from accounts where identifier = 'agent_creator')
    on conflict (account_id) do nothing;
  else
    insert into accounts (account_id, identifier, account_type)
    select '900000000000000077', 'agent_creator', 2
    where not exists (select 1 from accounts where identifier = 'agent_creator')
    on conflict (account_id) do nothing;
  end if;
end $$;

do $$
begin
  if not exists (select 1 from accounts where identifier = 'agent_creator') then
    raise exception 'failed to create canonical default assistant account agent_creator';
  end if;
end $$;

update accounts
set account_type = 2,
    updated_at = now()
where identifier = 'agent_creator';

insert into profiles (account_id, display_name, name, gender, region, avatar_media_id, avatar_url)
select account_id, 'AI 助手', 'agent_creator', 0, '', '', ''
from accounts
where identifier = 'agent_creator'
on conflict (account_id) do update
set display_name = excluded.display_name,
    name = excluded.name,
    updated_at = now();

update profiles p
set display_name = 'AI 助手',
    name = 'agent_creator',
    updated_at = now()
from accounts a
where a.account_id = p.account_id
  and a.identifier like 'agent_creator_legacy_%';

insert into agents (agent_id, account_id, name, description, status, created_by)
select account_id, account_id, 'agent_creator', 'Default general AI assistant', 'active', account_id
from accounts
where identifier = 'agent_creator'
on conflict (account_id) do update
set name = excluded.name,
    description = excluded.description,
    status = excluded.status,
    updated_at = now();

update agents ag
set name = 'agent_creator',
    description = 'Legacy agent_father migrated to canonical agent_creator.',
    status = 'archived',
    updated_at = now()
from accounts a
where a.account_id = ag.account_id
  and a.identifier like 'agent_creator_legacy_%';

insert into agent_prompts (name, description, content, variables_schema_json, version, status, created_by)
select
  'agent_creator_default_system_prompt',
  'System prompt for the default agent_creator assistant',
  '你是一个通用 AI 助手，回答应准确、简洁、友好。你可以帮助用户解释概念、比较方案、整理信息、生成文本和提供编程/产品建议。不要编造事实；不确定时说明不确定并给出可验证的下一步。',
  '{}'::jsonb,
  'v1',
  'active',
  account_id
from accounts
where identifier = 'agent_creator'
on conflict (name, version) do update
set description = excluded.description,
    content = excluded.content,
    variables_schema_json = excluded.variables_schema_json,
    status = excluded.status,
    updated_at = now();

insert into agent_prompt_bindings (agent_id, prompt_id, created_by)
select ag.agent_id, p.prompt_id, a.account_id
from accounts a
join agents ag on ag.account_id = a.account_id
join agent_prompts p on p.name = 'agent_creator_default_system_prompt' and p.version = 'v1'
where a.identifier = 'agent_creator'
on conflict (agent_id, prompt_id) do update
set updated_at = now();

insert into friendships (account_id, friend_account_id, status)
select human.account_id, assistant.account_id, 2
from accounts human
cross join accounts assistant
where human.account_type::text in ('1', 'user')
  and assistant.identifier = 'agent_creator'
  and human.account_id <> assistant.account_id
on conflict (account_id, friend_account_id) do update
set status = excluded.status,
    updated_at = now();

insert into friendships (account_id, friend_account_id, status)
select assistant.account_id, human.account_id, 2
from accounts human
cross join accounts assistant
where human.account_type::text in ('1', 'user')
  and assistant.identifier = 'agent_creator'
  and human.account_id <> assistant.account_id
on conflict (account_id, friend_account_id) do update
set status = excluded.status,
    updated_at = now();
