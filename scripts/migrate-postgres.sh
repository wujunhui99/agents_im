#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS_DIR="${ROOT_DIR}/db/migrations"

if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
  echo "missing migrations directory: ${MIGRATIONS_DIR}" >&2
  exit 1
fi

usage() {
  cat >&2 <<'MSG'
Usage:
  scripts/migrate-postgres.sh [--host-psql]

Environment:
  DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required with --host-psql.

The migrator records applied files in schema_migrations and skips already
applied versions. If an applied file's checksum changes, migration fails so
published migration drift is visible before deploy.
MSG
}

HOST_PSQL=0
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if [[ "${1:-}" == "--host-psql" ]]; then
  HOST_PSQL=1
  shift
fi
if [[ $# -gt 0 ]]; then
  usage
  exit 1
fi

psql_host() {
  local dsn="$1"
  shift
  psql "${dsn}" -v ON_ERROR_STOP=1 "$@"
}

psql_exec() {
  if [[ "${HOST_PSQL}" -eq 1 ]]; then
    psql_host "${DSN}" "$@"
  else
    docker compose exec -T postgres \
      psql -U "${POSTGRES_USER:-agents_im}" -d "${POSTGRES_DB:-agents_im}" -v ON_ERROR_STOP=1 "$@"
  fi
}

apply_migration_file() {
  local migration="$1"
  if [[ "${HOST_PSQL}" -eq 1 ]]; then
    psql_host "${DSN}" -1 -f "${migration}"
  else
    docker compose exec -T postgres \
      psql -U "${POSTGRES_USER:-agents_im}" -d "${POSTGRES_DB:-agents_im}" -v ON_ERROR_STOP=1 -1 \
      < "${migration}"
  fi
}

quote_sql_literal() {
  local value="$1"
  printf "'%s'" "${value//\'/\'\'}"
}

migration_is_legacy_applied() {
  local applied_checksum="$1"
  [[ "${applied_checksum}" == fixture-legacy-checksum-* ]]
}

if [[ "${HOST_PSQL}" -eq 1 ]]; then
  DSN="${DATABASE_URL:-${AGENTS_IM_POSTGRES_DSN:-}}"
  if [[ -z "${DSN}" ]]; then
    echo "DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for --host-psql" >&2
    exit 1
  fi
else
  cd "${ROOT_DIR}"
  docker compose up -d postgres
fi

psql_exec <<'SQL'
create table if not exists schema_migrations (
  version text primary key,
  checksum text not null,
  applied_at timestamptz not null default now()
);
SQL

shopt -s nullglob
migrations=("${MIGRATIONS_DIR}"/*.sql)
if ((${#migrations[@]} == 0)); then
  echo "no SQL migrations found in ${MIGRATIONS_DIR}" >&2
  exit 1
fi

for migration in "${migrations[@]}"; do
  version="$(basename "${migration}")"
  checksum="$(sha256sum "${migration}" | awk '{print $1}')"
  version_sql="$(quote_sql_literal "${version}")"
  checksum_sql="$(quote_sql_literal "${checksum}")"

  applied_checksum="$(psql_exec -Atc "select checksum from schema_migrations where version = ${version_sql};")"
  if [[ -n "${applied_checksum}" ]]; then
    if [[ "${applied_checksum}" != "${checksum}" ]]; then
      if migration_is_legacy_applied "${applied_checksum}"; then
        echo "migration ${version}: applying because legacy database recorded a pre-ledger checksum"
      else
        cat >&2 <<MSG
migration checksum mismatch for ${version}
  applied: ${applied_checksum}
  current: ${checksum}
Refusing to continue because a published migration appears to have changed.
Create a new migration instead of modifying an applied one.
MSG
        exit 1
      fi
    else
      echo "migration ${version}: already applied"
      continue
    fi
  fi

  echo "migration ${version}: applying"
  apply_migration_file "${migration}"
  psql_exec -c "insert into schema_migrations (version, checksum) values (${version_sql}, ${checksum_sql}) on conflict (version) do update set checksum = excluded.checksum, applied_at = now();"
done
