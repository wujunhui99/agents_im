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
  echo "[backend-verify] START ${name} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  "$@"
  echo "[backend-verify] END ${name} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

run_timeout_step() {
  local name="$1"
  local duration="$2"
  shift 2
  echo "[backend-verify] START ${name} timeout=${duration} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  timeout "${duration}" "$@"
  echo "[backend-verify] END ${name} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

run_step "install minimal apt dependencies" apt_get_cached ca-certificates ripgrep python3-yaml

run_step "cache diagnostics" bash -c 'go env GOMODCACHE GOCACHE GOPATH; command -v goctl >/dev/null 2>&1 && goctl --version || true'

run_timeout_step "gofmt check" 2m bash -c '
  mapfile -t go_files < <(find . -name "*.go" -print)
  if ((${#go_files[@]} > 0)); then
    unformatted="$(gofmt -l "${go_files[@]}")"
    if [[ -n "${unformatted}" ]]; then
      printf "%s\n" "${unformatted}"
      exit 1
    fi
  fi
'

run_timeout_step "go mod download" 4m go mod download
run_timeout_step "go test non-integration packages" 4m bash -c '
  packages=$(go list ./... | grep -v "/tests$")
  go test ${packages} -timeout=4m
'
run_timeout_step "verify static" 3m bash scripts/verify-static.sh

# Backend verification intentionally keeps PR/push checks focused and bounded.
# PostgreSQL integration runs in its own pipeline, so this job skips the top-level
# ./tests package and avoids duplicate integration work. Network-heavy checks
# such as goctl/protoc installs, docker compose validation, and Markdown external
# link checks are excluded from the hot path; run them manually or in a separate
# non-blocking job when API/proto/docs contracts change.
