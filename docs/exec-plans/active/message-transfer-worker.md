# message-transfer-worker

状态：Completed

## 背景

Message Service 第一阶段同步完成消息写入、会话 seq、拉取和已读状态。后续 Message Outbox 与 Kafka/Redpanda 分支会提供真实事件来源；本任务先落地 Message Transfer worker 的第一阶段骨架、接口、内存实现和验证。

## 目标

- 新增 `cmd/message-transfer` worker 入口和 `etc/message-transfer.yaml`。
- 在 `internal/transfer` 定义 `MessageEvent`、`Envelope`、consumer、dispatcher、worker、retry/backoff 和幂等 hook。
- 提供 in-memory consumer/dispatcher/noop fixture，让测试不依赖外部 Kafka、Redpanda、PostgreSQL 或 Gateway。
- 更新架构文档、设计文档索引和静态验证脚本。

## 非目标

- 不删除 Message Service 现有同步 send/pull/read 行为。
- 不实现真实 Kafka/Redpanda 消费。
- 不实现真实 Gateway push fanout、离线推送或 delivery ACK。
- 不变更 main/develop 或合并其他分支。

## 任务拆分

- [x] 阅读 `AGENTS.md`、`ARCHITECTURE.md` 和指定消息链路设计文档。
- [x] 设计 worker-facing event、consumer、dispatcher、retry 和 idempotency contract。
- [x] 实现 `internal/transfer` worker skeleton 与内存/noop 实现。
- [x] 增加 worker 单元测试覆盖消费、dispatch、retryable failure、幂等和取消。
- [x] 增加 `cmd/message-transfer` 与 `etc/message-transfer.yaml`。
- [x] 更新 `docs/design-docs/message-transfer-worker.md`、`ARCHITECTURE.md` 和 docs index。
- [x] 更新 `scripts/verify-static.sh` 覆盖 worker code/cmd/config/docs/tests。
- [x] 完成强制验证、提交并推送。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 使用 `internal/transfer` 作为 worker 包名 | 与 Message Service、Gateway、Presence 现有包边界一致，避免耦合到 repository 或 gateway internals |
| 2026-04-29 | 默认 runtime 使用 memory consumer + noop dispatcher | 满足第一阶段可启动、可测试，不阻塞 outbox/kafka/gateway 分支 |
| 2026-04-29 | Worker 接口显式暴露 `MarkRetry`/`MarkFailed` 和 `RetryDecision` | 后续 OutboxRepository 或 retry topic 需要持久化 backoff、attempt 和 next attempt time |
| 2026-04-29 | 幂等 hook 独立为 `IdempotencyStore` | 允许当前用内存实现，集成时替换为 Redis 或 durable store |

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
npx --yes markdown-link-check@3.13.7 --config .github/markdown-link-check.json $(find . -name "*.md" -not -path "./.git/*" -not -path "./.ai-context/*" -not -path "./docs/references/*" -print)
git status --short --branch
```

## 风险与回滚

- 风险：真实 Kafka/Redpanda 分支的 envelope 字段可能有命名差异。缓解：当前 `Envelope` 保留 topic/key/partition/offset/raw payload，集成时只需添加 adapter。
- 风险：进程内幂等不能跨 worker 重启。缓解：生产集成前替换 `IdempotencyStore` 实现。
- 回滚：移除 `internal/transfer`、`cmd/message-transfer`、`etc/message-transfer.yaml` 以及对应文档/静态检查条目，不影响 Message Service 同步行为。

## 结果记录

- `goctl --version`：通过，版本 `1.10.1 linux/amd64`。
- `for f in api/*.api; do goctl api validate -api "$f"; done`：通过，5 个 API 文件均 `api format ok`。
- `gofmt -w $(find . -name "*.go" -print)`：已执行。
- `go test ./...`：通过，包含 `internal/transfer` 和既有 `tests`。
- `bash scripts/verify-static.sh`：通过，输出 `static verification passed`。
- `docker compose config`：通过。
- `markdown-link-check`：通过，新增 `message-transfer-worker.md` 链接有效。
- `git status --short --branch`：提交前仅包含本任务新增/修改文件。
