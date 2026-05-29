# Stage 4e — 清理 agent 残部

> 前置：Stage 2（pkg/ 建立后 pythonexec 已迁）、Stage 3 完成  
> 后置：可与其他 Stage 4 任务并发  
> 风险：中（agent 服务 api 已迁，只需收尾 rpc 内部逻辑）  
> 预计 PR 数：2

---

## 背景

agent 服务当前状态（见 01-project-structure.md §1）：
- `service/agent/api/` — 已迁
- `cmd/agent-api` — 已迁，走 entry
- 但 `internal/agentim/`、`internal/agentruntime/`、`internal/agenteval/`、`internal/logic/agent*` 仍在 `internal/`
- `internal/agent/pythonexec` → Stage 2 已迁到 `pkg/pythonexec/`
- agent rpc 服务（`cmd/agent-rpc`）尚不存在，需新建（00-decisions，见 docs/refactor/04-*.md §3）

---

## 目标布局

```
service/agent/
├── api/                             # 已存在
└── rpc/
    ├── agent.proto                  # agent rpc 协议
    ├── agent.go
    ├── agentclient/
    ├── entry/  etc/
    └── internal/
        ├── trigger/                 # ← 原 internal/agentim/
        ├── orchestrator/            # ← 原 internal/agentruntime/
        ├── hosting/                 # ← 原 internal/logic/agentlogic*（ai hosting 相关）
        ├── imadapter/               # ← 原 internal/agentim/ 中 IM 集成部分
        ├── audit/                   # ← 原 internal/agenteval/
        ├── runtime/                 # 执行器（调 pkg/pythonexec）
        └── svc/

cmd/agent-rpc/
└── main.go                          # 新建
```

---

## 步骤

### 步骤 14.1 — 建 service/agent/rpc 骨架

```bash
mkdir -p service/agent/rpc/internal/{trigger,orchestrator,hosting,imadapter,audit,runtime,svc}
mkdir -p service/agent/rpc/entry
mkdir -p cmd/agent-rpc
```

### 步骤 14.2 — 识别各 internal/ 目录的归属

| 旧路径 | 目标子目录 | 说明 |
|--------|-----------|------|
| `internal/agentim/` | `imadapter/` + `trigger/` | IM 消息触发 + agent 启动 |
| `internal/agentruntime/` | `orchestrator/` | agent 执行编排 |
| `internal/agenteval/` | `audit/` | 评估/审计 |
| `internal/logic/agentlogic*.go` | `hosting/` | AI hosting 逻辑 |
| `internal/logic/aihostinglogic*.go` | `hosting/` | 同上 |
| `internal/logic/agent_registry*.go` | `trigger/` | agent 注册 |
| `internal/logic/agent_definition*.go` | `trigger/` | agent 定义 |

### 步骤 14.3 — 搬源码

```bash
git mv internal/agentim/       service/agent/rpc/internal/imadapter/
git mv internal/agentruntime/  service/agent/rpc/internal/orchestrator/
git mv internal/agenteval/     service/agent/rpc/internal/audit/
# 拆分 logic 文件（按上表归类）
```

### 步骤 14.4 — 建 cmd/agent-rpc

```go
package main

import (
    "flag"
    "github.com/wujunhui99/agents_im/service/agent/rpc/entry"
)

func main() {
    configFile := flag.String("f", "etc/agent-rpc.yaml", "config file")
    flag.Parse()
    entry.Start(*configFile)
}
```

### 步骤 14.5 — 清理命名混乱（TD-10）

Stage 2 中 `internal/agent/pythonexec` → `pkg/pythonexec/` 已完成。  
此步骤确认 `internal/agent/` 目录已完全清空并删除：

```bash
test -z "$(ls internal/agent/ 2>/dev/null)" && git rm -r internal/agent/ || echo "internal/agent/ not empty"
```

---

## 验收

```bash
# 无旧 internal/agentim 等 import
! grep -r 'agents_im/internal/agentim\|agents_im/internal/agentruntime\|agents_im/internal/agenteval' \
  --include='*.go' .

# internal/agent/ 已删
test ! -d internal/agent/

go build ./cmd/agent-api/ ./cmd/agent-rpc/
```
