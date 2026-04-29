# websocket-gateway

状态：Completed

## 背景

Gateway contract 已定义 WebSocket command 到 Message Service 能力的第一阶段映射，但仓库还缺少可启动的 WebSocket 长连接入口。当前优先级任务 C 需要在不阻塞 CI、Redis Presence、Kafka/Push 并行分支的前提下实现真实 Gateway 第一阶段。

## 目标

- 新增 `cmd/gateway-ws` 和 `etc/gateway-ws.yaml`。
- 实现 `GET /ws` WebSocket upgrade。
- 使用现有 JWT token 校验 handshake，支持 `Authorization` header 和 `token` query param。
- 实现内存 connection manager，按 `user_id` 支持多端 `connection_id`。
- 实现 `heartbeat`、`send_message`、`pull_messages`、`get_conversation_seqs`、`mark_conversation_read` command router。
- 通过现有 `MessageLogic` 调用消息业务能力，不在 Gateway 内重写 message 持久化、seq、幂等或 read-state 规则。
- 添加不依赖外部 PG/Redis 的测试。
- 更新架构、设计文档和静态验证脚本。

## 非目标

- 不实现 Kafka fanout、Push worker、offline push 或 delivery ACK。
- 不接入 Redis Presence；只保留 `PresenceReporter` 衔接点。
- 不修改 docker-compose Redis 或 CI workflow。
- 不改 user/auth/friends/groups/message 业务逻辑边界。
- 不删除现有 `internal/gateway/contract.go`。
- 不改 goctl REST/RPC scaffold。

## 任务拆分

- [x] 读取 AGENTS、架构、go-zero skill/reference、Gateway/Message/JWT/Postgres 设计和 gateway contract。
- [x] 设计 Gateway 边界与执行计划。
- [x] 实现 WebSocket server、JWT handshake、connection manager 和 command router。
- [x] 新增 `cmd/gateway-ws/main.go` 与 `etc/gateway-ws.yaml`。
- [x] 新增 WebSocket Gateway 测试。
- [x] 更新 `docs/design-docs/websocket-gateway.md`、`ARCHITECTURE.md`、设计文档索引和静态验证脚本。
- [x] 执行强制验证并记录结果。
- [ ] 提交并推送 `feature/websocket-gateway`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 使用独立 `cmd/gateway-ws`，不改 goctl REST API 入口 | WebSocket 长连接不适合通过 goctl REST scaffold 强行生成，且需求允许独立 Gateway server。 |
| 2026-04-29 | Gateway 默认使用 memory message repository，可配置 PostgreSQL | 测试不依赖外部 PG/Redis，同时保留与现有持久化 repository 的生产式接入路径。 |
| 2026-04-29 | Canonical frame 使用 `request_id/type/data/error`，兼容 `requestId/command/payload` 输入 | 满足本任务 envelope 要求，同时不破坏既有 Gateway contract。 |
| 2026-04-29 | Redis Presence 只保留接口衔接点 | Presence 由其他分支并行处理，当前分支避免修改 docker-compose Redis 和跨分支实现。 |

## 验证方式

- `goctl --version`
- `for f in api/*.api; do goctl api validate -api "$f"; done`
- `gofmt -w $(find . -name "*.go" -print)`
- `go test ./...`
- `bash scripts/verify-static.sh`
- `docker compose config`
- `git status --short --branch`

## 风险与回滚

- 风险：memory 模式下 Gateway 与 message-api 是不同进程内存，不能共享消息状态。
  回滚/缓解：生产式联调使用 `StorageDriver: postgres`，后续 develop 集成接入 Redis Presence 和 Push/Kafka。
- 风险：当前只返回 command ACK，不做在线投递 ACK。
  回滚/缓解：delivery ACK 留给 Kafka/Push worker 分支，不影响本阶段 send/pull/read 补偿链路。
- 风险：WebSocket origin 策略仍采用 gorilla 默认校验。
  回滚/缓解：正式前增加配置化 allowed origins。

## 结果记录

已实现 Gateway 第一阶段代码、测试与文档。

验证结果：

- `goctl --version`：通过，`goctl version 1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 `.api` 文件均 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：通过。
- `go test ./...`：通过。
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`。
- `docker compose config`：通过。
