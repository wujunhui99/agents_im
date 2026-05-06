#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DSN="${DATABASE_URL:-${AGENTS_IM_POSTGRES_DSN:-}}"
HOST_PSQL=0

psql_exec() {
  if [[ "${HOST_PSQL}" -eq 1 ]]; then
    psql "${DSN}" -v ON_ERROR_STOP=1 "$@"
  else
    docker compose exec -T postgres \
      psql -U "${POSTGRES_USER:-agents_im}" -d "${POSTGRES_DB:-agents_im}" -v ON_ERROR_STOP=1 "$@"
  fi
}

psql_file() {
  local sql_file="$1"
  if [[ "${HOST_PSQL}" -eq 1 ]]; then
    psql "${DSN}" -v ON_ERROR_STOP=1 -f "${sql_file}"
  else
    docker compose exec -T postgres \
      psql -U "${POSTGRES_USER:-agents_im}" -d "${POSTGRES_DB:-agents_im}" -v ON_ERROR_STOP=1 \
      < "${sql_file}"
  fi
}

if [[ -n "${DSN}" ]]; then
  HOST_PSQL=1
else
  if ! command -v docker >/dev/null 2>&1; then
    echo "DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required when Docker is unavailable" >&2
    exit 1
  fi

  echo "DATABASE_URL or AGENTS_IM_POSTGRES_DSN not set; using Docker Compose postgres" >&2
  docker compose up -d postgres
  for _ in {1..30}; do
    if docker compose exec -T postgres \
      pg_isready -U "${POSTGRES_USER:-agents_im}" -d "${POSTGRES_DB:-agents_im}" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

psql_exec <<'SQL'
drop table if exists schema_migrations cascade;
drop table if exists agent_trigger_idempotency cascade;
drop table if exists accounts cascade;
SQL

psql_file tests/fixtures/postgres/legacy_agent_trigger_idempotency.sql
if [[ "${HOST_PSQL}" -eq 1 ]]; then
  bash scripts/migrate-postgres.sh --host-psql
else
  bash scripts/migrate-postgres.sh
fi

psql_exec <<'SQL'
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

  if not exists (
    select 1 from schema_migrations
    where version = '003_agent_conversation_hosting.sql'
  ) then
    raise exception 'migration 003 was not recorded during legacy upgrade';
  end if;

  if not exists (
    select 1 from pg_indexes
    where schemaname = current_schema()
      and tablename = 'agent_trigger_idempotency'
      and indexname = 'agent_trigger_idempotency_trigger_idx'
  ) then
    raise exception 'agent_trigger_idempotency_trigger_idx was not created';
  end if;
end $$;
SQL
