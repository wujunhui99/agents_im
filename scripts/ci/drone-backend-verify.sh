#!/usr/bin/env bash
set -euo pipefail

export PATH=/tmp/go/bin:"${HOME}/go/bin:${PATH}"

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

run_timeout_step "go test ./..." 10m go test ./... -timeout=10m
run_timeout_step "verify static" 5m bash scripts/verify-static.sh

# Backend verification intentionally avoids network-heavy or deploy-irrelevant
# checks such as goctl/protoc tool installs, API/proto validation, docker compose
# validation, and Markdown external-link checks. Those can be run manually or in
# separate non-blocking jobs when contracts/docs change.
