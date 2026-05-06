#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DSN="${DATABASE_URL:-${AGENTS_IM_POSTGRES_DSN:-}}"
if [[ -z "${DSN}" ]]; then
  echo "DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required for migration verification" >&2
  exit 1
fi

bash scripts/migrate-postgres.sh --host-psql
bash scripts/migrate-postgres.sh --host-psql
