# Stage 4b — 建 service/message（最大块）

> 前置：Stage 3 完成  
> 后置：可与其他 Stage 4 任务并发，但 Stage 4f（repository 清理）部分依赖本任务  
> 风险：极高（核心消息链路，影响 message-api 和 message-rpc）  
> 预计 PR 数：3-4（骨架 → logic 搬迁 → rpc 切换 → message-api 瘦身）

---

## 背景

message 是未迁的最大域：
- `cmd/message-api/main.go` 60 行手动装配，从 `internal/repository`、`internal/logic`、`internal/agentim`、`internal/auth/repository` 一把抓
- `cmd/message-rpc` 从 `internal/rpcgen/message` 拿生成的 pb
- `proto/message.proto`（Stage 3 已 mv 到 `service/message/rpc/message.proto`）
- 详细设计见 `docs/refactor/07-*.md`（message 扩展为 10 个 RPC）

---

## 目标布局

```
service/message/
├── api/
│   ├── message.api
│   ├── message.go
│   ├── entry/
│   ├── etc/message-api.yaml
│   └── internal/{config,handler,logic,middleware,svc,types}/
└── rpc/
    ├── message.proto                # Stage 3 已就位
    ├── message.go
    ├── messageclient/               # 生成的 client
    ├── messagepb/                   # 生成的 pb.go
    ├── entry/  etc/
    └── internal/{config,logic,server,svc,repository}/
```

---

## 步骤

### 步骤 11.1 — 建骨架

```bash
mkdir -p service/message/{api,rpc}/internal/{config,logic,svc}
mkdir -p service/message/api/internal/{handler,middleware,types}
mkdir -p service/message/rpc/internal/{server,repository}
mkdir -p service/message/{api,rpc}/entry
```

### 步骤 11.2 — 重新生成 message pb（基于 Stage 3 的 proto）

```bash
cd service/message/rpc
goctl rpc protoc message.proto --go_out=. --go-grpc_out=. --zrpc_out=.
```

删 `internal/rpcgen/message`（import 替换后）：

```bash
MODULE="github.com/wujunhui99/agents_im"
find . -type f -name '*.go' \
  | xargs sed -i '' \
    "s|${MODULE}/internal/rpcgen/message|${MODULE}/service/message/rpc/messageclient|g"
git rm -r internal/rpcgen/message
```

### 步骤 11.3 — 搬 message logic

```bash
git mv internal/logic/message/   service/message/api/internal/logic/
git mv internal/handler/message/ service/message/api/internal/handler/
git mv internal/servicecontext/message/ service/message/api/internal/svc/
```

更新 import path，调整 package 声明。

### 步骤 11.4 — 搬 message repository

将 `internal/repository/` 中属于 message 域的文件拆出：
- `message_repository.go`、`postgres_message*.go` → `service/message/rpc/internal/repository/`

（其余 repository 文件留给各自域的 Stage 4 任务处理，最终由 Stage 4f 删空）

### 步骤 11.5 — message-api 瘦身

`cmd/message-api/main.go` 改为 5 行调 entry：

```go
package main

import (
    "flag"
    "github.com/wujunhui99/agents_im/service/message/api/entry"
)

func main() {
    configFile := flag.String("f", "etc/message-api.yaml", "config file")
    flag.Parse()
    entry.Start(*configFile)
}
```

### 步骤 11.6 — gozero_routes.go 中 message 路由下沉

将 `RegisterMessageGoZeroHandlers`（或等价）迁到 `service/message/api/internal/handler/routes.go`，从 `gozero_routes.go` 中删除。

---

## 验收

```bash
# message-api/main.go ≤ 10 行
[ $(wc -l < cmd/message-api/main.go) -le 10 ] && echo "OK"

# 无旧 rpcgen/message import
! grep -r 'agents_im/internal/rpcgen/message' --include='*.go' .

# 无旧 logic/message import
! grep -r 'agents_im/internal/logic/message' --include='*.go' .

go build ./cmd/message-api/ ./cmd/message-rpc/
```
