#!/usr/bin/env bash
set -euo pipefail

export PATH=/tmp/go/bin:"${HOME}/go/bin:${PATH}"

if command -v apt-get >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl protobuf-compiler ripgrep python3-yaml
else
  echo "apt-get is required by the Drone backend verification image" >&2
  exit 1
fi

install -d -m 0755 /etc/apt/keyrings
. /etc/os-release

if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
fi

if ! command -v node >/dev/null 2>&1 || ! node -e 'process.exit(Number(process.versions.node.split(".")[0]) >= 20 ? 0 : 1)' >/dev/null 2>&1; then
  curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
fi

apt-get update
DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce-cli docker-compose-plugin nodejs

go install github.com/zeromicro/go-zero/tools/goctl@v1.10.1
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

goctl --version
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version

for f in api/*.api; do
  goctl api validate -api "$f"
done
for f in service/*/api/*.api; do
  goctl api validate -api "$f"
done

mapfile -t go_files < <(find . -name "*.go" -print)
if ((${#go_files[@]} > 0)); then
  unformatted="$(gofmt -l "${go_files[@]}")"
  if [[ -n "${unformatted}" ]]; then
    printf '%s\n' "${unformatted}"
    exit 1
  fi
fi

go test ./...
bash scripts/verify-static.sh
docker compose config

mapfile -t md_files < <(
  find . -name "*.md" \
    -not -path "./.git/*" \
    -not -path "./.ai-context/*" \
    -not -path "./docs/references/*" \
    -print
)
if ((${#md_files[@]} > 0)); then
  npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json "${md_files[@]}"
fi
