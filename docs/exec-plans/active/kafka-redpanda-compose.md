# kafka-redpanda-compose

状态：Active

## 背景

消息链路当前由 Message Service 同步写 PostgreSQL，并为 Gateway 返回 command ACK。后续 outbox、Message Transfer worker、Gateway fanout 和 Push 链路需要先共享 Kafka-compatible topic、event schema 和 producer abstraction，避免并行分支各自定义事件格式。

## 目标

- 在 [`../../../docker-compose.yml`](../../../docker-compose.yml) 中增加单节点 Redpanda。
- 在 [`../../../.env.example`](../../../.env.example) 中增加 `KAFKA_*` 本地开发变量。
- 在 [`../../../internal/messaging/event.go`](../../../internal/messaging/event.go) 和 [`../../../internal/messaging/producer.go`](../../../internal/messaging/producer.go) 中增加消息事件契约和 producer abstraction。
- 新增 [`../../design-docs/kafka-message-events.md`](../../design-docs/kafka-message-events.md) 记录 topic、schema、delivery semantics 和链路边界。
- 更新架构索引、设计文档索引和 [`../../../scripts/verify-static.sh`](../../../scripts/verify-static.sh)。

## 非目标

- 不新增 outbox 表。
- 不实现 transfer worker 消费循环。
- 不实现 Gateway push fanout 或 delivery ACK。
- 不删除 PostgreSQL / Redis compose 配置。
- 不把 Message Service 当前同步写路径切换到 Kafka。

## 任务拆分

- [x] Task 1：读取 AGENTS、架构和消息/Gateway/Presence 设计文档，确认边界。
- [x] Task 2：新增 Redpanda compose 服务与 `KAFKA_*` 环境变量。
- [x] Task 3：新增 `MessageEvent` schema、producer interface、no-op/in-memory producer 和 Kafka producer。
- [x] Task 4：新增 Kafka message event 设计文档并更新索引。
- [x] Task 5：扩展静态验证脚本覆盖 Redpanda、Kafka env、事件契约、producer 和文档。
- [x] Task 6：执行强制验证、修复问题并提交推送。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 使用 Redpanda 单节点作为本地 Kafka-compatible broker | 本地开发无需 ZooKeeper，Compose 配置简单，后续 Kafka 客户端可直接复用 |
| 2026-04-29 | 默认 topic 为 `message.events.v1`，producer key 为 `conversation_id` | 保持同一会话内事件分区有序，不承诺跨会话全局顺序 |
| 2026-04-29 | 保留 Message Service 同步写 PostgreSQL，不接入 Kafka 发布 | 本分支只提供契约和中间件；outbox 与消费链路由其他分支并行实现 |
| 2026-04-29 | Redpanda integration test 默认跳过 | `go test ./...` 不依赖本地 broker，开发者可显式启用集成测试 |

## 验证方式

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

## 风险与回滚

- Redpanda 端口冲突：通过 `REDPANDA_KAFKA_PORT` 和 `REDPANDA_ADMIN_PORT` 调整端口。
- 重复事件：后续消费者必须按 `event_id` 幂等，并兼容重复 `server_msg_id` 或 `conversation_id + seq`。
- Topic schema 演进：新增字段必须保持向后兼容；破坏性变更需要新 topic 版本。
- 回滚方式：移除 Redpanda compose 服务、`KAFKA_*` env、`internal/messaging` 包和对应文档/static check。

## 结果记录

2026-04-29 本分支完成 Redpanda compose、Kafka env、消息事件 contract、producer abstraction、设计文档和静态验证更新。验证结果：

- `goctl --version`：通过，`goctl version 1.10.1 linux/amd64`
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 API specs 均 `api format ok`
- `gofmt -w $(find . -name "*.go" -print)`：通过
- `go test ./...`：通过
- `bash scripts/verify-static.sh`：通过，`static verification passed`
- `docker compose config`：通过，包含 postgres、redis、redpanda
- `markdown-link-check`：通过
