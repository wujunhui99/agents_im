#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${script_dir}/cache-env.sh"
source "${script_dir}/apt-cache.sh"
ci_cache_summary

export GOPROXY="${GOPROXY:-https://goproxy.cn,https://proxy.golang.org,direct}"
export GONOSUMDB="${GONOSUMDB:-}"

run_step() {
  local name="$1"
  shift
  echo "[postgres-integration] START ${name} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  "$@"
  echo "[postgres-integration] END ${name} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

: "${DATABASE_URL:?DATABASE_URL is required and must point to the Drone postgres service}"

run_step "install postgres client" apt_get_cached postgresql-client
run_step "cache diagnostics" go env GOMODCACHE GOCACHE GOPATH

run_step "wait for postgres" bash -c '
  for i in $(seq 1 30); do
    if pg_isready -h postgres -U agents_im -d agents_im >/dev/null 2>&1; then
      exit 0
    fi
    if [[ "$i" == "30" ]]; then
      echo "postgres service did not become ready" >&2
      exit 1
    fi
    sleep 2
  done
'

run_step "go mod download" go mod download
run_step "migrate postgres" bash scripts/migrate-postgres.sh --host-psql
run_step "go test integration" go test -tags=integration ./tests -timeout=4m
