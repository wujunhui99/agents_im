# Stage 1 — 删除明显残骸

> 前置：无  
> 后置：Stage 2 可以直接跟上  
> 风险：低（纯删除，不动业务逻辑）  
> 预计 PR 数：1

---

## 目标

清除已迁服务留下的过渡产物，不触碰任何业务逻辑。

---

## ✅ 已完成（PR #305）

### 步骤 1 — 删 `internal/rpcgen/{friends,groups}`

`cmd/friends-rpc` 和 `cmd/groups-rpc` 已走 `service/<domain>/rpc/entry`，这两个目录是纯孤儿代码（只有目录内自引用，无外部调用者）。

```bash
git rm -r internal/rpcgen/friends internal/rpcgen/groups
go build ./...  # ✅ 通过
```

> `internal/rpcgen/message` 暂留（message 服务未迁，Stage 4b 处理）。

---

## ⛔ 移出本 Stage（原因：仍有活跃引用）

### 步骤 2 — 删 `internal/servicecontext/{auth,user,friends,groups}/`

**实际情况**：`gozero_routes.go` + `internal/handler/*` + `internal/logic/*` 仍大量引用这四个 svc，这整条链路都由 `cmd/message-api` 驱动。`service/<domain>/api/internal/svc/` 虽然存在，但 message-api 并未切过去。

**归入**：Stage 4b（message-api 迁完后，整条 internal/handler + internal/logic + internal/servicecontext 一并清除）。

### 步骤 3 — 删 outbox 相关（D1）

**实际情况**：`cmd/message-transfer/main.go` 仍调用 `transfer.NewOutboxEventConsumer`，`internal/transfer/outbox_consumer.go` 引用 `internal/outboxpublisher`。

**归入**：Stage 4d（transfer 服务迁移时一并处理）。

### 步骤 4 — 删 `internal/types/` 和 `internal/model/`

**实际情况**：`internal/types` 被 80+ 个 handler/logic 文件引用；`internal/model` 被 repository、logic、agentim 等大量文件引用。

**归入**：Stage 4b/4f（各域迁完后跟着清）。

### 步骤 5 — `etc/` 单源化

**实际情况**：`service/*/etc/` 与根 `etc/` 内容不同（前者是 goctl 模板含 Etcd 配置，后者是真实部署配置），不能直接删。需要先在文档中明确约定两者角色，再决定是否清理。

**归入**：独立讨论，暂不处理。

---

## 验收

```bash
test ! -d internal/rpcgen/friends && test ! -d internal/rpcgen/groups
go build ./...
```
