# Stage 4a — 拆 admin-api（TD-1）

> 前置：Stage 3 完成  
> 后置：可与其他 Stage 4 任务并发  
> 风险：高（拆分运行中服务的路由和初始化逻辑）  
> 预计 PR 数：2（建服务骨架 + 切流量卸载 message-api）

---

## 背景

`cmd/message-api/main.go` 同时 import `adminsvc` 和 `messagesvc`，`internal/handler/gozero_routes.go` 把 admin 路由注册到 message-api 的 go-zero Server。admin 没有独立服务边界，无法独立部署/扩容。

---

## 目标布局

```
service/admin/api/
├── admin.api                        # ← 原 api/admin.api
├── admin.go
├── entry/
│   └── entry.go
├── etc/admin-api.yaml
└── internal/
    ├── config/
    ├── handler/
    │   └── routes.go                # ← 原 internal/handler/admin/
    ├── logic/                       # ← 原 internal/logic/admin*
    ├── middleware/
    ├── svc/
    │   └── service_context.go       # ← 原 internal/servicecontext/admin/
    └── types/

cmd/admin-api/
└── main.go                          # 5 行，调 service/admin/api/entry.Start
```

---

## 步骤

### 步骤 10.1 — 建 service/admin/api 骨架

```bash
mkdir -p service/admin/api/internal/{config,handler,logic,middleware,svc,types}
mkdir -p service/admin/api/entry
mkdir -p cmd/admin-api
```

用 goctl 生成或手写 entry/entry.go（参考 service/auth/api/entry/entry.go）。

### 步骤 10.2 — 搬源码

```bash
# handler
git mv internal/handler/admin service/admin/api/internal/handler/admin

# logic
git mv internal/logic/admin*.go   service/admin/api/internal/logic/
git mv internal/logic/admin/      service/admin/api/internal/logic/   # 若有子目录

# bootstrap/svc
git mv internal/adminbootstrap/   service/admin/api/internal/svc/bootstrap/
git mv internal/servicecontext/admin/ service/admin/api/internal/svc/

# api 文件（若 Stage 3 已 mv 则跳过）
test -f api/admin.api && git mv api/admin.api service/admin/api/admin.api
```

更新各文件 package 路径和 import path：

```bash
MODULE="github.com/wujunhui99/agents_im"
find service/admin -type f -name '*.go' \
  | xargs sed -i '' \
    -e "s|${MODULE}/internal/handler/admin|${MODULE}/service/admin/api/internal/handler/admin|g" \
    -e "s|${MODULE}/internal/logic/admin|${MODULE}/service/admin/api/internal/logic|g" \
    -e "s|${MODULE}/internal/adminbootstrap|${MODULE}/service/admin/api/internal/svc/bootstrap|g" \
    -e "s|${MODULE}/internal/servicecontext/admin|${MODULE}/service/admin/api/internal/svc|g"
```

### 步骤 10.3 — 建 cmd/admin-api/main.go

```go
package main

import (
    "flag"
    "github.com/wujunhui99/agents_im/service/admin/api/entry"
)

func main() {
    configFile := flag.String("f", "etc/admin-api.yaml", "config file")
    flag.Parse()
    entry.Start(*configFile)
}
```

### 步骤 10.4 — message-api 卸下 admin 依赖

在 `cmd/message-api/main.go` 中删除所有 admin 相关 import 和初始化代码。  
在 `internal/handler/gozero_routes.go` 中删除 `RegisterAdminGoZeroHandlers`（或等价函数）。

```bash
# 验证 message-api 无 admin import
grep 'admin' cmd/message-api/main.go && echo "FAIL" || echo "OK"
```

### 步骤 10.5 — 配置与部署

- 复制并调整 `etc/admin-api.yaml`（端口、JWT 配置等）
- 在 `deploy/k8s/` 添加 admin-api deployment 和 service 清单
- Drone CI pipeline 加入 admin-api build 步骤

---

## 验收

```bash
# admin 服务独立启动
go run cmd/admin-api/main.go -f etc/admin-api.yaml &
curl -f http://localhost:<admin-port>/health

# message-api 无 admin 依赖
! grep -r 'agents_im/internal/.*admin\|agents_im/service/admin' cmd/message-api/ --include='*.go'

# 编译通过
go build ./cmd/admin-api/ ./cmd/message-api/
```
