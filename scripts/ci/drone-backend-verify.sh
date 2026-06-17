#!/usr/bin/env bash
# 后端验证：goctl api validate + gofmt + go test + CI 单测 + docker compose
# config + 静态门禁。Markdown 死链检查由独立 node 步骤执行；protoc/protoc-gen-*
# 已删（验证流程无消费方，proto 再生成是 dev 时操作）。GOMODCACHE/GOCACHE/GOBIN
# 由 .drone.yml 指到 host 缓存卷，goctl 命中版本即跳过安装。
set -euo pipefail

GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
export PATH="${GOBIN}:${PATH}"

if command -v apt-get >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates curl gnupg ripgrep python3-yaml
else
  echo "apt-get is required by the Drone backend verification image" >&2
  exit 1
fi

ensure_docker_compose() {
  if docker compose version >/dev/null 2>&1; then
    return 0
  fi

  install -m 0755 -d /etc/apt/keyrings
  if [[ ! -s /etc/apt/keyrings/docker.asc ]]; then
    curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
    chmod a+r /etc/apt/keyrings/docker.asc
  fi

  . /etc/os-release
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian ${VERSION_CODENAME} stable" \
    >/etc/apt/sources.list.d/docker.list

  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce-cli docker-compose-plugin
}

GOCTL_VERSION=1.10.1
if ! command -v goctl >/dev/null 2>&1 || ! goctl --version | grep -qF "${GOCTL_VERSION}"; then
  go install "github.com/zeromicro/go-zero/tools/goctl@v${GOCTL_VERSION}"
fi
goctl --version

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
python3 -m unittest discover -s tests/ci -t tests/ci -p 'test_*.py'
ensure_docker_compose
docker compose config -q
bash scripts/verify-static.sh
