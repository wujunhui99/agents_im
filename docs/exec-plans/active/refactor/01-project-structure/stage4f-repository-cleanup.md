# Stage 4f — internal/repository 按域拆散

> 前置：Stage 4b（message）、4c（gateway）、4d（transfer/push）各自将本域 repository 搬走  
> 后置：Stage 5  
> 风险：中（纯 mv + import 替换，各域迁移后收尾）  
> 预计 PR 数：1（各域迁走后统一删目录）

---

## 背景

`internal/repository/` 现有 50+ 文件扁平堆放，包含账户、消息、群组、agent、媒体、conversation_ai_hosting、agent_hosting、agent_registry、feedback、task_report、delivery_attempt 等所有域的数据访问层。

目标：每个域的 repository 随着 Stage 4a-4e 分批迁走，最终 `internal/repository/` 整目录删除（00-decisions D10）。

---

## 各域 repository 归属

| 文件模式 | 归属域 | 目标位置 |
|----------|--------|----------|
| `*account*`, `*user*` | user | `service/user/rpc/internal/repository/` |
| `*message*`, `*outbox*` | message | `service/message/rpc/internal/repository/`（outbox 已 Stage 1 删） |
| `*friend*` | friends | `service/friends/rpc/internal/repository/` |
| `*group*` | groups | `service/groups/rpc/internal/repository/` |
| `*agent*`, `*agent_hosting*`, `*agent_registry*` | agent | `service/agent/rpc/internal/repository/` |
| `*media*` | message 或独立 | `service/message/rpc/internal/repository/`（待确认） |
| `*feedback*` | agent | `service/agent/rpc/internal/repository/` |
| `*task_report*` | agent | `service/agent/rpc/internal/repository/` |
| `*delivery_attempt*` | transfer | `service/transfer/internal/repository/` |
| `*conversation_ai*` | agent | `service/agent/rpc/internal/repository/` |
| `memory.go`, `*_memory.go` | 各域 | 拆到对应域的 `repository/memory/` 子目录 |
| `postgres_*.go` | 各域 | 拆到对应域的 `repository/postgres/` 子目录 |

---

## 步骤

### 步骤 15.1 — 统计当前文件

```bash
ls -1 internal/repository/ | sort
```

### 步骤 15.2 — 按域 mv（各 Stage 4 任务各自负责本域部分）

此步骤是收尾确认，确保前置 Stage 4 任务已将本域文件搬走。

逐个检查：

```bash
# 确认各 Stage 4 任务已将本域文件迁出
ls internal/repository/ | grep -E 'friend|group|message|agent|transfer|user'
```

### 步骤 15.3 — 处理残留跨域文件

若有跨域共用的文件（如通用的 `db.go`、`tx.go`），判断归属：
- 若仅一个域用 → 归该域
- 若多个域用 → 提升到 `pkg/database/`（需确认不违反 pkg 约束）

### 步骤 15.4 — 删 internal/repository/

```bash
# 最终确认目录为空或只剩无归属文件
ls internal/repository/
# 全部处理后
git rm -r internal/repository/
```

---

## 验收

```bash
# internal/repository/ 已删
test ! -d internal/repository/

# 无旧 internal/repository import
! grep -r 'agents_im/internal/repository' --include='*.go' .

go build ./...
```
