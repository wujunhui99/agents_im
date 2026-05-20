#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${script_dir}/cache-env.sh"
source "${script_dir}/apt-cache.sh"
ci_cache_summary

apt_get_cached ca-certificates curl protobuf-compiler ripgrep python3-yaml

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

apt_get_cached docker-ce-cli docker-compose-plugin nodejs

install_go_tool() {
  local bin_name="$1"
  local module_spec="$2"
  if command -v "${bin_name}" >/dev/null 2>&1; then
    echo "[backend-verify] cached ${bin_name}: $(${bin_name} --version 2>/dev/null || ${bin_name} version 2>/dev/null || true)"
    return 0
  fi
  echo "[backend-verify] installing ${module_spec}"
  go install "${module_spec}"
}

install_go_tool goctl github.com/zeromicro/go-zero/tools/goctl@v1.10.1
install_go_tool protoc-gen-go google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
install_go_tool protoc-gen-go-grpc google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.1

goctl --version
protoc --version
protoc-gen-go --version
protoc-gen-go-grpc --version

echo "[backend-verify] go env cache dirs"
go env GOMODCACHE GOCACHE GOPATH

for f in api/*.api; do
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

go mod download
packages="$(go list ./... | grep -v '/tests$' || true)"
if [[ -n "${packages}" ]]; then
  # shellcheck disable=SC2086
  go test ${packages} -timeout=4m
fi
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
  npx --yes --cache "${npm_config_cache}" markdown-link-check@3.13.7 --config .github/markdown-link-check.json "${md_files[@]}"
fi
