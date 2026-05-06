#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DSN="${DATABASE_URL:-${AGENTS_IM_POSTGRES_DSN:-}}"
if [[ -z "${DSN}" ]]; then
  echo "DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for old-schema upgrade verification" >&2
  exit 1
fi

psql "${DSN}" -v ON_ERROR_STOP=1 <<'SQL'
drop table if exists schema_migrations cascade;
drop table if exists agent_trigger_idempotency cascade;
drop table if exists accounts cascade;
SQL

psql "${DSN}" -v ON_ERROR_STOP=1 -f tests/fixtures/postgres/legacy_agent_trigger_idempotency.sql
bash scripts/migrate-postgres.sh --host-psql

psql "${DSN}" -v ON_ERROR_STOP=1 <<'SQL'
do $$
begin
  if not exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'trigger_server_msg_id'
  ) then
    raise exception 'agent_trigger_idempotency.trigger_server_msg_id was not created';
  end if;

  if exists (
    select 1 from information_schema.columns
    where table_name = 'agent_trigger_idempotency'
      and column_name = 'trigger_message_id'
  ) then
    raise exception 'legacy trigger_message_id column still exists';
  end if;

  if not exists (
    select 1 from agent_trigger_idempotency
    where idempotency_key = 'fixture-key'
      and trigger_server_msg_id = 'msg_fixture'
      and status = 'succeeded'
  ) then
    raise exception 'legacy row was not upgraded as expected';
  end if;
end $$;
SQL
