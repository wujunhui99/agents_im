# Stage 5 — 收尾验证

> 前置：Stage 1-4 全部完成  
> 风险：低（只读验证 + CI 规则新增）  
> 预计 PR 数：1

---

## 目标

确认顶层目录干净，cmd 全部瘦身，无残留 import，添加 CI lint 规则防止回退。

---

## 验收清单

### 步骤 17 — 顶层目录检查

```bash
# A. 顶层禁止目录已消失
test ! -d internal && echo "internal: OK" || echo "internal: FAIL"
test ! -d api      && echo "api: OK"      || echo "api: FAIL"
test ! -d proto    && echo "proto: OK"    || echo "proto: FAIL"
test ! -d rpcgen   && echo "rpcgen: OK"   || echo "rpcgen: FAIL"
```

### 步骤 18 — cmd 全部 ≤ 10 行

```bash
# B. 所有 cmd/*/main.go 不超过 10 行
for f in cmd/*/main.go; do
  lines=$(wc -l < "$f")
  if [ "$lines" -gt 10 ]; then
    echo "FAIL: $f has $lines lines"
  else
    echo "OK: $f ($lines lines)"
  fi
done
```

### 步骤 19 — 无残留 import

```bash
MODULE="github.com/wujunhui99/agents_im"

# C. 无旧顶层 import
! grep -r "\"${MODULE}/internal" --include='*.go' . && echo "internal import: OK"
! grep -r "\"${MODULE}/api"      --include='*.go' . && echo "api import: OK"
! grep -r "\"${MODULE}/proto"    --include='*.go' . && echo "proto import: OK"

# D. pkg → service 单向依赖（pkg 内不 import service）
! grep -r "\"${MODULE}/service" pkg/ --include='*.go' && echo "pkg→service: OK"

# E. 跨域 rpc 互调检查（service/<domain>/rpc/internal/logic 不直接 import 其他域 rpc/internal/logic）
# 示例：friends rpc logic 不能 import groups rpc logic
! grep -r "${MODULE}/service/groups/rpc/internal" service/friends/rpc/internal/logic/ --include='*.go'
! grep -r "${MODULE}/service/friends/rpc/internal" service/groups/rpc/internal/logic/ --include='*.go'
```

---

## CI lint 规则（步骤 19 附加）

在 `scripts/verify-static.sh` 中加入（或新建 `scripts/verify-layout.sh`）：

```bash
#!/usr/bin/env bash
set -e
MODULE="github.com/wujunhui99/agents_im"

echo "=== Layout lint ==="

# 禁止顶层 internal/api/proto/rpcgen 出现
for dir in internal api proto rpcgen; do
  if [ -d "$dir" ]; then
    echo "FAIL: top-level $dir/ must not exist"
    exit 1
  fi
done

# cmd/*/main.go 行数
for f in cmd/*/main.go; do
  lines=$(wc -l < "$f")
  if [ "$lines" -gt 10 ]; then
    echo "FAIL: $f has $lines lines (max 10)"
    exit 1
  fi
done

# 无旧 import
if grep -r "\"${MODULE}/internal" --include='*.go' . -l 2>/dev/null | grep -v '^Binary'; then
  echo "FAIL: found imports from ${MODULE}/internal"
  exit 1
fi

# pkg 单向依赖
if grep -r "\"${MODULE}/service" pkg/ --include='*.go' -l 2>/dev/null; then
  echo "FAIL: pkg/ must not import service/"
  exit 1
fi

echo "=== Layout lint: PASS ==="
```

在 `.drone.yml` 中加入：

```yaml
- name: layout-lint
  image: golang:1.22-alpine
  commands:
    - sh scripts/verify-layout.sh
```

---

## 最终业务验证

1. 每个域 `cmd/<service>-api/main.go` 启动后只挂自己的路由前缀
2. admin-api 独立部署，访问 admin 路由只打 admin-api 进程
3. `service/<domain>/rpc/<domain>.proto` 是该域 RPC 契约单源
4. `pkg/` 内不出现任何域名词（`pkg/friends/`、`pkg/message/` 等）

```bash
# 全量构建
go build ./...

# 运行现有集成测试
go test ./tests/...
```
