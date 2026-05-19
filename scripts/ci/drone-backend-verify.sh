#!/usr/bin/env bash
set -euo pipefail

export PATH=/tmp/go/bin:"${HOME}/go/bin:${PATH}"

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

if command -v apt-get >/dev/null 2>&1; then
  run_timeout_step "apt update base" 5m apt-get update
  run_timeout_step "apt install base tools" 5m env DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl protobuf-compiler ripgrep python3-yaml
else
  echo "apt-get is required by the Drone backend verification image" >&2
  exit 1
fi

run_step "prepare apt keyrings" install -d -m 0755 /etc/apt/keyrings
. /etc/os-release

if ! command -v docker >/dev/null 2>&1; then
  run_timeout_step "download docker apt key" 2m curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  run_step "chmod docker apt key" chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
fi

if ! command -v node >/dev/null 2>&1 || ! node -e 'process.exit(Number(process.versions.node.split(".")[0]) >= 20 ? 0 : 1)' >/dev/null 2>&1; then
  run_timeout_step "install nodesource setup" 5m bash -c 'curl -fsSL https://deb.nodesource.com/setup_20.x | bash -'
fi

run_timeout_step "apt update final" 5m apt-get update
run_timeout_step "apt install docker node" 5m env DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce-cli docker-compose-plugin nodejs

run_timeout_step "install goctl" 5m go install github.com/zeromicro/go-zero/tools/goctl@v1.10.1
run_timeout_step "install protoc-gen-go" 5m go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
run_timeout_step "install protoc-gen-go-grpc" 5m go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

run_step "tool versions" bash -c 'goctl --version && protoc --version && protoc-gen-go --version && protoc-gen-go-grpc --version'

run_step "goctl api validate" bash -c 'for f in api/*.api; do goctl api validate -api "$f"; done'

run_step "gofmt check" bash -c '
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
run_timeout_step "docker compose config" 3m docker compose config

mapfile -t md_files < <(
  find . -name "*.md" \
    -not -path "./.git/*" \
    -not -path "./.ai-context/*" \
    -not -path "./docs/references/*" \
    -print
)
if ((${#md_files[@]} > 0)); then
  echo "[backend-verify] markdown files=${#md_files[@]}"
  run_timeout_step "markdown link check" 5m npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json "${md_files[@]}"
fi
