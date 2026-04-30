# gateway-push-delivery

状态：Completed

归档日期：2026-05-01

## 背景

WebSocket Gateway 已实现 JWT handshake、connection manager 和 send/pull/read command router。第二优先级并行任务 D 需要新增 Gateway push delivery 第一阶段接口和本进程内 fanout，让未来 Message Transfer worker 可以把消息事件主动下发到在线 WebSocket 连接。Outbox、Kafka/Redpanda、Transfer worker、跨进程路由和 Redis Presence 订阅由其他分支处理。

## 目标

- 新增 `internal/gateway/delivery` dispatcher 契约与 delivery event/message/result 结构。
- 新增 `internal/gateway/ws` in-memory dispatcher，使用 `ConnectionManager` 向同一用户所有在线连接写 server push event。
- 在 WebSocket `Server` 暴露 server-side push 方法，供未来 worker 调用。
- 支持 `message_received` / `message_delivered` push event，payload 包含 `server_msg_id`、`conversation_id`、`seq`、`sender_id` 和内容元数据。
- offline recipient 返回明确 `offline` 状态，不 panic。
- 保留现有 WebSocket command request/response 行为。
- 新增测试并更新文档、架构索引和静态验证。

## 非目标

- 不实现真实 Kafka/Redpanda consumer。
- 不实现 durable outbox reader 或 Transfer worker。
- 不实现跨 Gateway 进程 fanout。
- 不实现 Redis Presence 订阅或跨实例路由。
- 不实现 offline push、delivery ACK worker 或消息重试队列。
- 不删除现有 send/pull/read/heartbeat command。

## 任务拆分

- [x] 读取 AGENTS、架构、Gateway、Redis Presence、Gateway Message contract 和 WebSocket 代码。
- [x] 定义 dispatcher interface、delivery event/message/result 和状态枚举。
- [x] 实现 in-memory connection fanout 与 offline/failed 状态返回。
- [x] 在 WebSocket Server 暴露 `PushToUser`、`PushToConversation` 和 `DeliveryDispatcher`。
- [x] 新增 WebSocket push delivery 测试。
- [x] 更新 Gateway push delivery 设计文档、架构文档、设计文档索引和静态验证脚本。
- [x] 执行强制验证并记录结果。
- [x] 提交并推送 `feature/gateway-push-delivery`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 将稳定 dispatcher 契约放在 `internal/gateway/delivery`，WebSocket in-memory 实现放在 `internal/gateway/ws` | 未来 Message Transfer worker 只依赖 Gateway delivery 契约，不需要直接操作 WebSocket connection 内部。 |
| 2026-04-29 | `DeliverToConversation` 接收已解析 recipient user IDs | Gateway 不拥有会话成员关系，成员解析应由 Message Service、IM Core 或 Transfer worker 所属链路负责。 |
| 2026-04-29 | push event 使用无 `request_id` 的 `type/data` envelope | 避免与 command ACK 混淆，同时保持现有 command response path 不变。 |
| 2026-04-29 | local offline 返回 result status，不返回 fatal error | 本阶段只有本进程连接视图；未来可结合 Redis Presence 判断是否需要跨实例转发。 |

## 验证方式

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- `npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)`
- `git status --short --branch`

## 风险与回滚

- 风险：当前 `offline` 只表示本 Gateway 进程内没有连接，不代表全局离线。
  缓解：文档明确后续需要结合 Redis Presence 做跨实例路由。
- 风险：server push event 与 command response 在同一 WebSocket 上交错。
  缓解：push event 不带 `request_id`，客户端用 `type` 和 `request_id` 区分；测试覆盖 push 后 command ACK 仍可用。
- 风险：关闭连接后 manager 残留 stale connection。
  缓解：close 测试覆盖 unregister；write 失败时 dispatcher 也会 unregister。
- 回滚：移除 `internal/gateway/delivery`、`internal/gateway/ws/delivery.go`、Server push 方法和对应测试/文档，恢复静态验证脚本条目。

## 结果记录

2026-05-01 状态对齐：

- 当前 `main` 已包含 Gateway delivery dispatcher 契约、同实例 WebSocket fanout、`message_received` / `message_delivered` push event envelope 和 offline/failed result 分类。
- 该计划已无 active 剩余任务，因此从 active 归档到 completed。
- 本计划没有实现真实 Kafka/Redpanda consumer、durable outbox reader、跨实例 Gateway RPC、offline push、delivery ACK worker 或 read receipt push ACK；这些仍不得被解释为已完成。
- 本次只做文档状态对齐，未启动真实依赖，也未声称端到端验证。

已完成 Gateway push delivery 第一阶段接口、in-memory fanout、server push 方法、测试、文档和静态验证脚本更新。

验证结果：

- `goctl --version`：通过，`goctl version 1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 `.api` 文件均 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：通过。
- `go test ./...`：通过。
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`。
- `docker compose config`：通过。
- `npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json ...`：通过。
- `git status --short --branch`：提交前位于 `feature/gateway-push-delivery`，仅包含本任务变更。
