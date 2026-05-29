# Stage 2 — 建立 `pkg/`（顶层 internal 退役第一步）

> 前置：Stage 1 完成  
> 后置：Stage 3 可以直接跟上  
> 风险：中（需全仓 import path 替换，无逻辑改动）  
> 预计 PR 数：1

---

## 目标

将跨服务可重用的基础设施从 `internal/` 批量迁移到 `pkg/`，仅改 import path，不改逻辑。迁完后 `pkg/` 内不允许 import `service/...`（单向依赖：`service → pkg`）。

---

## 迁移表

| 旧路径 | 新路径 | 备注 |
|--------|--------|------|
| `internal/apperror/` | `pkg/apperror/` | 纯 mv |
| `internal/response/` | `pkg/response/` | 纯 mv |
| `internal/ctxuser/` | `pkg/ctxuser/` | 纯 mv |
| `internal/config/` | `pkg/config/` | 纯 mv |
| `internal/observability/` | `pkg/observability/` | 纯 mv |
| `internal/llmobs/` | `pkg/llmobs/` | 纯 mv |
| `internal/health/` | `pkg/health/` | 纯 mv |
| `internal/idgen/` | `pkg/idgen/` | 纯 mv |
| `internal/messaging/` | `pkg/messaging/` | 纯 mv |
| `internal/presence/` | `pkg/presence/` | 纯 mv（D4：仅作在线状态查询） |
| `internal/objectstorage/` | `pkg/objectstorage/` | 纯 mv |
| `internal/agent/pythonexec/` | `pkg/pythonexec/` | 扁平化，去掉 `agent/` 中间层 |

---

## 步骤

### 步骤 6 — 批量 git mv

```bash
# 建 pkg 目录
mkdir -p pkg

# 批量 mv（逐一执行，避免遗漏）
git mv internal/apperror      pkg/apperror
git mv internal/response      pkg/response
git mv internal/ctxuser       pkg/ctxuser
git mv internal/config        pkg/config
git mv internal/observability pkg/observability
git mv internal/llmobs        pkg/llmobs
git mv internal/health        pkg/health
git mv internal/idgen         pkg/idgen
git mv internal/messaging     pkg/messaging
git mv internal/presence      pkg/presence
git mv internal/objectstorage pkg/objectstorage
git mv internal/agent/pythonexec pkg/pythonexec
```

### 替换全仓 import path

```bash
MODULE="github.com/wujunhui99/agents_im"

pkgs=(apperror response ctxuser config observability llmobs health idgen messaging presence objectstorage)
for pkg in "${pkgs[@]}"; do
  find . -type f -name '*.go' \
    | xargs sed -i '' "s|${MODULE}/internal/${pkg}|${MODULE}/pkg/${pkg}|g"
done

# pythonexec 单独处理（路径从 internal/agent/pythonexec）
find . -type f -name '*.go' \
  | xargs sed -i '' "s|${MODULE}/internal/agent/pythonexec|${MODULE}/pkg/pythonexec|g"
```

### 验证

```bash
go build ./...
go vet ./...
# 确认无旧 import 残留
grep -r "${MODULE}/internal/apperror\|${MODULE}/internal/response\|${MODULE}/internal/config\|${MODULE}/internal/observability" \
  --include='*.go' . && echo "FAIL: old imports found" || echo "OK"
```

---

## 验收

```bash
# pkg/ 目录存在且有内容
ls pkg/apperror pkg/response pkg/config pkg/observability pkg/pythonexec

# 无旧路径 import
! grep -r 'agents_im/internal/apperror\|agents_im/internal/response\|agents_im/internal/ctxuser' \
  --include='*.go' .

# pkg 内无反向 import
! grep -r 'agents_im/service' pkg/ --include='*.go'

go build ./...
```
