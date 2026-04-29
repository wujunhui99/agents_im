# outbox-kafka-publisher

状态：Active

## 背景

Message Service 已在同步发送事务内写入 `message_outbox` 的 `message.created` 事件，Kafka-compatible `message.events.v1` schema 和 `messaging.Producer` 抽象也已存在。本任务补齐 outbox 到 Kafka/Redpanda 的发布模块，但不改变现有同步 send/pull/read 行为，也不实现 Kafka consumer、Gateway delivery 或 Redis presence routing。

## 目标

- 新增 `internal/outboxpublisher` 模块，依赖 `repository.OutboxRepository` 和 `messaging.Producer`。
- 使用 `worker_id`、batch limit、lock duration 轮询 pending outbox rows。
- 将 repository `message.created` payload 转换为 `messaging.MessageEvent` 的 `message.accepted` 事件。
- 通过 producer 发布到配置为 `message.events.v1` 的 topic，并保持 `conversation_id` 作为 partition key。
- publish 成功后 `MarkPublished`。
- publish 或 convert 失败后 `MarkFailed`，记录 retry metadata。
- 覆盖 success、retryable failure、malformed payload、context cancellation 的无外部依赖单元测试。
- 更新设计文档和静态验证。

## 非目标

- 不实现 Kafka consumer loop。
- 不接入 Message Transfer consumer。
- 不改 Gateway dispatcher 或 Gateway push delivery。
- 不实现 Redis presence routing。
- 不引入默认依赖 PostgreSQL、Redis、Kafka 的测试。

## 任务拆分

- [x] Task 1：阅读 AGENTS、ARCHITECTURE、go-zero skill、message chain/outbox/Kafka/transfer/gateway push 设计文档。
- [x] Task 2：梳理 `OutboxRepository`、`MessageCreatedOutboxPayload`、`messaging.MessageEvent` 和 `Producer` 契约。
- [x] Task 3：实现 `internal/outboxpublisher.Publisher` 的 batch poll、convert、publish、mark published、mark failed 和 cancellation 行为。
- [x] Task 4：新增无外部依赖单元测试。
- [x] Task 5：新增 `docs/design-docs/outbox-kafka-publisher.md` 并更新设计索引/架构入口。
- [x] Task 6：扩展 `scripts/verify-static.sh` 的 publisher module/docs 检查。
- [x] Task 7：运行 goctl validate、gofmt、go test、static verify、docker compose config。
- [x] Task 8：记录验证结果，提交并推送 feature branch。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | Publisher 只依赖 repository outbox 与 `messaging.Producer`，不新增 consumer 或 Gateway wiring | 保持本分支边界清晰，避免改变同步消息链路和 Gateway delivery 行为 |
| 2026-04-29 | Kafka event `event_id` 复用 outbox `event_id` | 保持跨 outbox retry 和 Kafka redelivery 的稳定幂等键 |
| 2026-04-29 | `message.created` 转换为 Kafka `message.accepted`，topic 由 Producer 配置为 `message.events.v1` | 匹配已接受的 Kafka event contract，同时让 publisher 不耦合具体 producer 实现 |
| 2026-04-29 | context cancellation 不调用 `MarkFailed` | 取消是 worker shutdown，不代表事件或 broker 失败；锁过期后可由后续 worker retry |
| 2026-04-29 | text content 发布为 JSON object `{"text":"..."}` | 满足 `messaging.MessageEventPayload.Content` 的 JSON contract，并保留文本语义 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
```

## 风险与回滚

- Publish 成功但 `MarkPublished` 前进程退出会导致重复 Kafka event；这是 at-least-once 语义，消费者必须按 `event_id` 去重。
- Malformed payload 会进入 retry/failed 路径，需要后续运维视图或 dead-letter 处理。
- 当前未提供生产入口和 metrics，部署 wiring 应在后续任务中补齐。
- 回滚方式：移除 `internal/outboxpublisher` 调用方即可停止发布；当前实现没有改变 Message Service 同步写路径。

## 结果记录

- `goctl --version`：通过，版本 `goctl version 1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 API specs 均 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：通过。
- `go test ./...`：通过；默认测试未依赖 live PostgreSQL/Redis/Kafka。
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`。
- `docker compose config`：通过。
