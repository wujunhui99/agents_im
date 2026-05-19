#!/usr/bin/env bash
set -euo pipefail

export PATH=/tmp/go/bin:"${HOME}/go/bin:${PATH}"
export GOPROXY="${GOPROXY:-https://goproxy.cn,https://proxy.golang.org,direct}"
export GONOSUMDB="${GONOSUMDB:-}"

run_timeout_step() {
  local name="$1"
  local duration="$2"
  shift 2
  echo "[backend-verify] START ${name} timeout=${duration} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  timeout "${duration}" "$@"
  echo "[backend-verify] END ${name} $(date -u +%Y-%m-%dT%H:%M:%SZ)"
}

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

run_timeout_step "go mod download" 8m go mod download
run_timeout_step "go test selected packages" 8m bash -c '
  packages=$(go list ./... | grep -v "/tests$")
  go test ${packages} -timeout=8m
'
run_timeout_step "verify static" 5m bash scripts/verify-static.sh

# Backend verification intentionally keeps PR/push checks focused and bounded.
# The slower end-to-end PostgreSQL test suite runs in the postgres-integration
# pipeline, so this job skips the top-level ./tests package to avoid duplicating
# the same cold module downloads and integration checks.
# Network-heavy/deploy-irrelevant checks such as goctl/protoc installs, API/proto
# validation, docker compose validation, and Markdown external-link checks can be
# run manually or in separate non-blocking jobs when contracts/docs change.
