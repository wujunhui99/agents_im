-- Operational copy of the deploy migration for Issue #8.
-- Safe to run repeatedly; it only repairs direct conversation participant state.
insert into user_conversation_states (account_id, conversation_id, last_read_seq, visible_start_seq)
select participant.account_id, t.conversation_id, 0, 0
from conversation_threads t
cross join lateral (
  values (t.single_account_a), (t.single_account_b)
) as participant(account_id)
where t.conversation_type = 1
  and participant.account_id <> ''
on conflict (account_id, conversation_id) do update
set visible_start_seq = 0,
    updated_at = case
      when user_conversation_states.visible_start_seq <> 0 then now()
      else user_conversation_states.updated_at
    end
where user_conversation_states.visible_start_seq <> 0;
