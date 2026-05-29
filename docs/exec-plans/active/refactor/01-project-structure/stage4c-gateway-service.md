# Stage 4c — 建 service/gateway/ws

> 前置：Stage 3 完成  
> 后置：可与其他 Stage 4 任务并发  
> 风险：高（WebSocket 长连接，切换时需保证在线用户不断连）  
> 预计 PR 数：2（骨架 + 切换）

---

## 背景

gateway 是 WebSocket 接入层，当前完全在 `internal/`：
- `internal/gateway/{ws,delivery}` — 连接管理、消息分发
- `internal/servicecontext/gateway/` — svc
- `cmd/gateway-ws/main.go` — 手动装配
- 根 `proto/` 中**没有** gateway proto（gateway 不对外暴露 gRPC，仅消费 Kafka）

见 `docs/refactor/` 中 03-message-pipeline 的 §7：gateway 应砍掉业务依赖，只做连接管理 + 消息路由，不直接调 DB。

---

## 目标布局

```
service/gateway/ws/
├── gateway.go
├── entry/
│   └── entry.go
├── etc/gateway-ws.yaml
└── internal/
    ├── config/
    ├── conn/                        # ← 原 internal/gateway/ws/
    ├── delivery/                    # ← 原 internal/gateway/delivery/
    ├── svc/
    │   └── service_context.go       # ← 原 internal/servicecontext/gateway/
    └── handler/

cmd/gateway-ws/
└── main.go                          # 5 行
```

> gateway 特殊形态：无 api/rpc 二分，只有 ws/。

---

## 步骤

### 步骤 12.1 — 建骨架

```bash
mkdir -p service/gateway/ws/internal/{config,conn,delivery,svc,handler}
mkdir -p service/gateway/ws/entry
```

### 步骤 12.2 — 搬源码

```bash
git mv internal/gateway/ws/        service/gateway/ws/internal/conn/
git mv internal/gateway/delivery/  service/gateway/ws/internal/delivery/
git mv internal/servicecontext/gateway/ service/gateway/ws/internal/svc/
```

更新 import path：

```bash
MODULE="github.com/wujunhui99/agents_im"
find service/gateway -type f -name '*.go' \
  | xargs sed -i '' \
    -e "s|${MODULE}/internal/gateway/ws|${MODULE}/service/gateway/ws/internal/conn|g" \
    -e "s|${MODULE}/internal/gateway/delivery|${MODULE}/service/gateway/ws/internal/delivery|g" \
    -e "s|${MODULE}/internal/servicecontext/gateway|${MODULE}/service/gateway/ws/internal/svc|g"
```

### 步骤 12.3 — 砍业务依赖（00-decisions D8）

按 03-message-pipeline §7 要求，gateway 不应直接调 DB 或 business logic：

- 检查 `internal/gateway/` 中是否有直接调 repository 的代码
- 如有，改为通过 Kafka 消费或通过 RPC 调用对应服务
- 确保 `service/gateway/ws/internal/svc/` 中只持有 Kafka consumer、Redis client（在线状态），不持有 DB connection

### 步骤 12.4 — cmd/gateway-ws 瘦身

```go
package main

import (
    "flag"
    "github.com/wujunhui99/agents_im/service/gateway/ws/entry"
)

func main() {
    configFile := flag.String("f", "etc/gateway-ws.yaml", "config file")
    flag.Parse()
    entry.Start(*configFile)
}
```

---

## 验收

```bash
# gateway-ws/main.go ≤ 10 行
[ $(wc -l < cmd/gateway-ws/main.go) -le 10 ] && echo "OK"

# 无旧 internal/gateway import
! grep -r 'agents_im/internal/gateway' --include='*.go' .
! grep -r 'agents_im/internal/servicecontext/gateway' --include='*.go' .

# gateway 不直接 import repository
! grep -r 'agents_im/internal/repository' service/gateway/ --include='*.go'

go build ./cmd/gateway-ws/
```
