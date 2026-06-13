-- D16 (00-decisions): the default assistant must carry the agent facet
-- (toPush=0) in its account_id, otherwise the agent-trigger judge skips its
-- agent-inbox runs and msgtransfer tries to push to it.
--
-- Migration 009 seeds agent_creator with the literal account_id
-- 900000000000000077, which under the D16 layout decodes to facet=3
-- (human / toPush=1). Rewrite it to a facet-agent constant:
--
--   322961408 = (tick=77 << 22) | (facet=agent(0) << 19) | (machine=0 << 10) | seq=0
--   decode -> facet=0 (agent, toPush=0), machine=0, seq=0, tick=77 (issue #77 nod)
--
-- The constant is far below runtime-minted account ids (current-epoch tick ≈ 1e10
-- -> id ≈ 6e16), so it cannot collide with generated ids.
--
-- This runs as part of the D16 data reset, so agent_creator only exists in the
-- seed tables below (no messages / credentials / group memberships yet). The one
-- enforced FK on the account id is agents.account_id -> accounts.account_id; it is
-- dropped and re-created around the rewrite (no ON UPDATE CASCADE).

do $$
declare
  old_id text := '900000000000000077';
  new_id text := '322961408';
begin
  if not exists (select 1 from accounts where account_id = old_id) then
    return; -- already rewritten, or a different/clean seed: nothing to do
  end if;
  if exists (select 1 from accounts where account_id = new_id) then
    raise exception 'D16 default-assistant target account_id % already exists', new_id;
  end if;

  alter table agents drop constraint if exists agents_account_id_fkey;

  update accounts             set account_id = new_id        where account_id = old_id;
  update profiles             set account_id = new_id        where account_id = old_id;
  update agents               set account_id = new_id        where account_id = old_id;
  update agents               set created_by = new_id        where created_by = old_id;
  update agent_prompts        set created_by = new_id        where created_by = old_id;
  update agent_prompt_bindings set created_by = new_id       where created_by = old_id;
  update agent_tools          set created_by = new_id        where created_by = old_id;
  update agent_tool_bindings  set created_by = new_id        where created_by = old_id;
  update friendships          set account_id = new_id        where account_id = old_id;
  update friendships          set friend_account_id = new_id where friend_account_id = old_id;

  alter table agents
    add constraint agents_account_id_fkey
    foreign key (account_id) references accounts(account_id) on delete restrict;
end $$;

do $$
begin
  if not exists (
    select 1 from accounts
    where identifier = 'agent_creator' and account_id = '322961408'
  ) then
    raise exception 'D16 default-assistant facet rewrite did not land (agent_creator account_id != 322961408)';
  end if;
end $$;
