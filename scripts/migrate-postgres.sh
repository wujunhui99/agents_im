#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS_DIR="${ROOT_DIR}/db/migrations"

if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
  echo "missing migrations directory: ${MIGRATIONS_DIR}" >&2
  exit 1
fi

if [[ "${1:-}" == "--host-psql" ]]; then
  DSN="${DATABASE_URL:-${AGENTS_IM_POSTGRES_DSN:-}}"
  if [[ -z "${DSN}" ]]; then
    echo "DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for --host-psql" >&2
    exit 1
  fi
  for migration in "${MIGRATIONS_DIR}"/*.sql; do
    psql "${DSN}" -v ON_ERROR_STOP=1 -f "${migration}"
  done
  exit 0
fi

cd "${ROOT_DIR}"
docker compose up -d postgres

for migration in "${MIGRATIONS_DIR}"/*.sql; do
  docker compose exec -T postgres \
    psql -U "${POSTGRES_USER:-agents_im}" -d "${POSTGRES_DB:-agents_im}" -v ON_ERROR_STOP=1 \
    < "${migration}"
done
