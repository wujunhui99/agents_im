#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

DSN="${DATABASE_URL:-${AGENTS_IM_POSTGRES_DSN:-}}"
if [[ -z "${DSN}" ]]; then
  cat >&2 <<'MSG'
DATABASE_URL or AGENTS_IM_POSTGRES_DSN is required.

Use a dedicated local/test PostgreSQL database, for example:
  export DATABASE_URL='postgres://agents_im:[REDACTED]@localhost:5432/agents_im_test?sslmode=disable'
  AGENTS_IM_CONFIRM_TRUNCATE=1 scripts/verify-postgres-local.sh
MSG
  exit 1
fi

python3 - "${DSN}" <<'PY'
import sys
from urllib.parse import urlparse

dsn = sys.argv[1]
parsed = urlparse(dsn)
host = (parsed.hostname or '').lower()
db = (parsed.path or '').lstrip('/').lower()
raw = dsn.lower()

blocked_markers = [
    'agenticim.xyz',
    'production',
    'prod',
    'rds.amazonaws.com',
    'aliyuncs.com',
    'azure.com',
    'neon.tech',
    'supabase.co',
]
if any(marker in raw for marker in blocked_markers):
    raise SystemExit('Refusing to run PostgreSQL integration against an obvious production/remote DSN. Use a dedicated local/test database.')

local_hosts = {'localhost', '127.0.0.1', '::1', 'host.docker.internal'}
private_prefixes = ('10.', '172.16.', '172.17.', '172.18.', '172.19.', '172.20.', '172.21.', '172.22.', '172.23.', '172.24.', '172.25.', '172.26.', '172.27.', '172.28.', '172.29.', '172.30.', '172.31.', '192.168.')
if host and host not in local_hosts and not host.startswith(private_prefixes):
    raise SystemExit(f'Refusing non-local/non-private PostgreSQL host: {host}. Use a local/test database.')

if db in {'postgres', 'template0', 'template1'} or 'prod' in db or 'production' in db:
    raise SystemExit(f'Refusing database name that is unsafe for test truncation: {db!r}. Use a dedicated *_test database.')
PY

export DATABASE_URL="${DSN}"

cat >&2 <<'MSG'
WARNING: PostgreSQL integration tests may truncate test tables and mutate data.
Use only a dedicated local/test database. Never use production.
MSG

if [[ "${AGENTS_IM_CONFIRM_TRUNCATE:-}" != "1" && "${CI:-}" != "true" ]]; then
  cat >&2 <<'MSG'
Set AGENTS_IM_CONFIRM_TRUNCATE=1 to confirm that this DSN points to a disposable local/test database.
MSG
  exit 1
fi

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required for local PostgreSQL verification" >&2
  exit 1
fi

bash scripts/migrate-postgres.sh --host-psql
go test -tags=integration ./tests -count=1
