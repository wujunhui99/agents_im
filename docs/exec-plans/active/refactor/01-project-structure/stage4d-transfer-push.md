# Stage 4d — 建 service/transfer + service/push

> 前置：Stage 3 完成  
> 后置：可与其他 Stage 4 任务并发  
> 风险：高（消息链路核心消费者，需保证消息不丢）  
> 预计 PR 数：2（transfer 拆出 + 新建 push）

---

## 背景

- `internal/transfer/` — Kafka consumer，负责消息落库和分发（00-decisions D3）
- `cmd/message-transfer` — 入口，当前手动装配
- push 服务当前不存在，需新建（00-decisions D3：从 transfer 拆出离线推送逻辑）
- 见 `docs/refactor/03-message-pipeline`

---

## 目标布局

```
service/transfer/
├── transfer.go
├── entry/  etc/
└── internal/{config,batcher,handler,svc}/

service/push/
├── push.go
├── entry/  etc/
└── internal/{config,handler,offlinepush,svc}/

cmd/message-transfer/main.go     # 5 行
cmd/push/main.go                 # 5 行（新增）
```

---

## 步骤

### 步骤 13.1 — 建 service/transfer 骨架

```bash
mkdir -p service/transfer/internal/{config,batcher,handler,svc}
mkdir -p service/transfer/entry
```

### 步骤 13.2 — 搬 internal/transfer/

```bash
git mv internal/transfer/ service/transfer/internal/handler/
# 或按实际子目录结构拆分到 batcher/、handler/
```

更新 import path：

```bash
MODULE="github.com/wujunhui99/agents_im"
find service/transfer -type f -name '*.go' \
  | xargs sed -i '' \
    "s|${MODULE}/internal/transfer|${MODULE}/service/transfer/internal/handler|g"
```

### 步骤 13.3 — 从 transfer 中拆出 push 逻辑

识别 `internal/transfer/` 中负责离线推送（APNs、FCM、小米推送等）的代码，移到 `service/push/internal/offlinepush/`。

transfer 保留：消息路由、落库、发 Kafka  
push 接管：离线推送、push gateway 调用

### 步骤 13.4 — 建 service/push 骨架并新建 cmd/push

```bash
mkdir -p service/push/internal/{config,handler,offlinepush,svc}
mkdir -p service/push/entry
mkdir -p cmd/push
```

```go
// cmd/push/main.go
package main

import (
    "flag"
    "github.com/wujunhui99/agents_im/service/push/entry"
)

func main() {
    configFile := flag.String("f", "etc/push.yaml", "config file")
    flag.Parse()
    entry.Start(*configFile)
}
```

### 步骤 13.5 — cmd/message-transfer 瘦身

```go
package main

import (
    "flag"
    "github.com/wujunhui99/agents_im/service/transfer/entry"
)

func main() {
    configFile := flag.String("f", "etc/message-transfer.yaml", "config file")
    flag.Parse()
    entry.Start(*configFile)
}
```

---

## 验收

```bash
# main 文件瘦
[ $(wc -l < cmd/message-transfer/main.go) -le 10 ] && echo "OK"
[ $(wc -l < cmd/push/main.go) -le 10 ] && echo "OK"

# 无旧 internal/transfer import
! grep -r 'agents_im/internal/transfer' --include='*.go' .

go build ./cmd/message-transfer/ ./cmd/push/
```
