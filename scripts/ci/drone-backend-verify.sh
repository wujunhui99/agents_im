#!/usr/bin/env bash
# 后端验证（#488 提速版）：goctl api validate + gofmt + go test + CI 单测 + 静态门禁。
# 工具链精简：docker compose config → compose-config 步骤；markdown-link-check →
# 独立 node 步骤（drone-markdown-link-check.sh）；protoc/protoc-gen-* 已删（验证流程
# 无消费方，proto 再生成是 dev 时操作）。GOMODCACHE/GOCACHE/GOBIN 由 .drone.yml 指到
# host 缓存卷，goctl 命中版本即跳过安装。
set -euo pipefail

GOBIN="${GOBIN:-$(go env GOPATH)/bin}"
export PATH="${GOBIN}:${PATH}"

if command -v apt-get >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y ripgrep python3-yaml
else
  echo "apt-get is required by the Drone backend verification image" >&2
  exit 1
fi

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
bash scripts/verify-static.sh
