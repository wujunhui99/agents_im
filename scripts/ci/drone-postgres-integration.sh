#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${script_dir}/cache-env.sh"
source "${script_dir}/apt-cache.sh"
ci_cache_summary

apt_get_cached postgresql-client

: "${DATABASE_URL:?DATABASE_URL is required and must point to the Drone postgres service}"

echo "[postgres-integration] go env cache dirs"
go env GOMODCACHE GOCACHE GOPATH

for i in $(seq 1 30); do
  if pg_isready -h postgres -U agents_im -d agents_im >/dev/null 2>&1; then
    break
  fi
  if [[ "$i" == "30" ]]; then
    echo "postgres service did not become ready" >&2
    exit 1
  fi
  sleep 2
done

go mod download
bash scripts/migrate-postgres.sh --host-psql
go test -tags=integration ./tests -timeout=4m
