# message-outbox

状态：Active

## 背景

Message Service 已同步完成消息写入、会话 seq、幂等和已读状态更新。Kafka、Message Transfer worker 和 Gateway Push delivery 由其他分支并行处理，因此本任务只补齐可靠事件源。

## 目标

- 新增 PostgreSQL `message_outbox` 表，记录可被后续 worker 轮询的 `message.created` 事件。
- 在 Message Service PostgreSQL send transaction 内写入消息后同步写 outbox。
- 新增 `OutboxRepository` 契约和 PG/memory 实现，支持 pending poll/lock、mark published、mark failed/retry。
- 新增默认 memory 测试和可选 PostgreSQL integration test。
- 更新架构与设计文档，并让 `scripts/verify-static.sh` 覆盖 outbox 契约。

## 非目标

- 不实现 Kafka producer/consumer。
- 不实现 Message Transfer worker。
- 不实现 Gateway push fanout 或 delivery ACK。
- 不删除现有同步消息写入、HTTP ACK 或 WebSocket command ACK 行为。

## 任务拆分

- [x] Task 1：阅读 `AGENTS.md`、`ARCHITECTURE.md`、go-zero 参考、消息链路/存储/PG/Gateway 设计。
- [x] Task 2：更新 PostgreSQL migration，新增 `message_outbox` schema、约束和索引。
- [x] Task 3：新增 outbox repository 契约、PG 实现和 memory 实现。
- [x] Task 4：在 PostgreSQL send transaction 内追加 `message.created` outbox row。
- [x] Task 5：新增 memory 单元测试和 PostgreSQL integration test。
- [x] Task 6：更新设计文档、架构索引和静态验证脚本。
- [x] Task 7：运行强制验证并提交推送。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 使用 `message_outbox` 表名 | 保持 Message Service 归属清晰，避免和后续跨服务 outbox 混淆。 |
| 2026-04-29 | event type 使用 `message.created` | 下游 Kafka/Transfer/Push 需要的是已持久化消息事件。 |
| 2026-04-29 | outbox insert 放在消息事务尾部 | 确保只有消息、幂等、会话和已读状态都成功后才暴露事件源。 |
| 2026-04-29 | PG poll 使用 `FOR UPDATE SKIP LOCKED` | 支持多个 worker 并行轮询，避免重复锁定同一 pending row。 |
| 2026-04-29 | memory repository 实现 outbox 而不是 no-op | 默认 `go test ./...` 能验证 send 产生 outbox，不依赖 PostgreSQL。 |

## 验证方式

必须运行：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
git status --short --branch
```

PostgreSQL integration test requires `DATABASE_URL` or `AGENTS_IM_POSTGRES_DSN` and remains build-tagged with `integration`.

## 风险与回滚

- 风险：worker 重试可能导致下游重复处理。缓解：事件包含 `event_id`，且 `(event_type, aggregate_type, aggregate_id)` 唯一，消费者应幂等。
- 风险：outbox insert 失败会让 send transaction 回滚。缓解：这是可靠事件源的预期语义，避免消息已接受但后续无法投递。
- 回滚：回退 migration 中 `message_outbox` 相关 DDL、outbox repository 文件、send transaction outbox insert、测试和文档更新。

## 结果记录

Generator 已完成：

- 新增 `message_outbox` migration schema、索引和约束。
- 新增 `OutboxRepository` 契约、PostgreSQL 实现和 memory 实现。
- `PostgresMessageRepository.CreateMessageIdempotent` 在同一 transaction 内写入 `message.created` outbox row。
- 默认 memory 测试验证 send 产生 outbox，并覆盖 poll、retry、publish。
- PostgreSQL integration test 覆盖 migration/table、事务内 outbox、poll/mark failed retry/mark published；当前本地环境未设置 `DATABASE_URL`/`AGENTS_IM_POSTGRES_DSN`，integration 测试按设计跳过。
- 更新 `message-outbox.md`、架构、PG/message storage 设计、文档索引和静态验证脚本。

验证记录：

- `goctl --version`：通过，版本 `1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过。
- `gofmt -w $(find . -name "*.go" -print)`：已执行。
- `go test ./...`：通过。
- `go test -tags=integration ./tests`：通过；未设置 PostgreSQL DSN 时跳过 integration 用例。
- `bash scripts/verify-static.sh`：通过。
- `docker compose config`：通过。
- markdown link check：通过。
