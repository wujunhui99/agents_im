# kafka-transfer-consumer

状态：Active

## 背景

Message Service 当前仍同步写 PostgreSQL 并返回 send/pull/read 结果。Kafka/Redpanda topic `message.events.v1` 和 Message Transfer worker skeleton 已存在，本任务把两者连接起来，让 Transfer worker 可以通过 Kafka consumer 接收 `message.accepted` 事件，同时不改变现有同步消息行为。

## 目标

- 在 [`../../../internal/transfer`](../../../internal/transfer) 下新增 Kafka `EventConsumer`。
- 解码 [`../../../internal/messaging/event.go`](../../../internal/messaging/event.go) 的 `messaging.MessageEvent` JSON，并映射为 `transfer.Envelope` / `transfer.MessageEvent`。
- 通过 [`../../../internal/config/config.go`](../../../internal/config/config.go) 和 [`../../../etc/message-transfer.yaml`](../../../etc/message-transfer.yaml) 支持 broker、topic 和 group 配置。
- 增加 Kafka consumer 单元测试和 Redpanda-gated integration test。
- 新增 [`../../design-docs/kafka-transfer-consumer.md`](../../design-docs/kafka-transfer-consumer.md)，并扩展 [`../../../scripts/verify-static.sh`](../../../scripts/verify-static.sh)。

## 非目标

- 不实现 outbox polling/publisher。
- 不实现 Gateway push dispatch、Redis presence routing 或离线 push。
- 不改变 Message Service 现有同步 send/pull/read 行为和既有测试。
- 不提交真实 secrets、tokens、passwords、DSNs 或 credentials。

## 任务拆分

- [x] 阅读 AGENTS、ARCHITECTURE、go-zero context、消息链路、Kafka/outbox/transfer/Gateway 设计文档。
- [x] 设计 Kafka consumer 到 `transfer.EventConsumer` 的 adapter 和 ack 语义。
- [x] 实现 Kafka consumer、message decode、offset commit seam 和 worker entry config wiring。
- [x] 增加单元测试覆盖 decode、invalid event、constructor、ack 和 config mapping。
- [x] 增加 Redpanda integration test，默认按 env gate 跳过。
- [x] 新增 Kafka transfer consumer 设计文档并更新文档索引/架构说明。
- [x] 扩展静态验证脚本覆盖 consumer module/docs。
- [x] 执行强制验证、修复问题、提交并推送。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 将 Kafka consumer 放在 `internal/transfer` | 它直接实现 `transfer.EventConsumer`，并只依赖 `internal/messaging` 的 canonical event schema |
| 2026-04-29 | 只消费 `message.accepted`，拒绝 `message.read` | 本分支负责消息投递触发，read event 的后续消费不属于 Gateway fanout |
| 2026-04-29 | `MarkSuccessful` 提交 Kafka offset，`MarkRetry`/`MarkFailed` 不提交 | 保持 at-least-once，不在没有 retry-topic 或 dead-letter policy 时静默丢弃事件 |
| 2026-04-29 | 默认 `Consumer.Driver` 保持 `memory` | 普通本地启动和 `go test ./...` 不依赖 live Kafka/Redpanda |

## 验证方式

必须执行：

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name "*.go" -print)
go test ./...
bash scripts/verify-static.sh
docker compose config
```

Integration test 手动执行：

```bash
docker compose up -d redpanda
KAFKA_REDPANDA_INTEGRATION=1 KAFKA_BROKERS=localhost:19092 go test ./internal/transfer
```

## 风险与回滚

- 风险：Kafka retry 当前没有延迟队列，`MarkRetry` 不提交 offset 只能保证重启或 rebalance 后 redelivery。后续需要 retry-topic、outbox-backed retry 或 dead-letter policy。
- 风险：当前 worker 使用内存幂等，跨进程/重启 redelivery 需要 Redis 或 durable store。
- 风险：poison event 会以 receive error 形式保留，不会自动 commit。后续 DLQ 策略需要明确记录和提交条件。
- 回滚：移除 `internal/transfer/kafka_consumer*`、transfer Kafka wiring、配置新增字段和对应 docs/static checks，不影响 Message Service 同步链路。

## 结果记录

2026-04-29 本分支完成 Kafka transfer consumer、config wiring、unit/integration tests、设计文档和静态验证更新。验证结果：

- `goctl --version`：通过，`goctl version 1.10.1 linux/amd64`
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 API 文件均 `api format ok`
- `gofmt -w $(find . -name "*.go" -print)`：已执行
- `go test ./...`：通过，Redpanda integration test 默认跳过
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`
- `docker compose config`：通过，包含 postgres、redis、redpanda
- `markdown-link-check` changed docs：通过
