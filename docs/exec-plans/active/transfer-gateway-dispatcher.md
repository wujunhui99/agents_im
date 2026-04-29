# transfer-gateway-dispatcher

状态：Active

## 背景

Message Transfer worker 已有 `DeliveryDispatcher`、幂等和重试接口，Gateway 已有 `internal/gateway/delivery.Dispatcher` 与 WebSocket in-memory fanout 契约。本任务负责把两侧接口桥接起来，让 transfer `message.accepted` 事件可以转为 Gateway `message_received` push 请求，同时不引入真实 Kafka、Outbox publisher、Redis 跨实例路由或远程 Gateway 网络调用。

## 目标

- 新增 `internal/transfer/gateway` adapter，实现 `transfer.DeliveryDispatcher`。
- 复用 `internal/gateway/delivery.Dispatcher`、`delivery.EventMessageReceived` 和 per-recipient delivery result。
- 支持 direct recipient 的 accepted-message 投递。
- 对 no recipients、offline、gateway error 和 failed recipient 做明确分类。
- 增加单元测试覆盖成功、offline、no recipients、幂等跳过重复 dispatch、`RetryDecision`。
- 更新设计文档、架构索引和静态验证脚本。

## 非目标

- 不实现真实 Kafka consumer。
- 不实现 Outbox publisher 或真实 outbox polling。
- 不实现 Redis cross-instance presence routing。
- 不对远程 Gateway 进程发起网络调用。
- 不改变现有同步 send/pull/read 行为和测试。

## 任务拆分

- [x] 阅读 AGENTS、ARCHITECTURE、go-zero 参考和消息链路/Gateway/Transfer 设计文档。
- [x] 实现 transfer 到 Gateway delivery 的 dispatcher adapter。
- [x] 为 transfer event 增加 Gateway delivery 所需的可选 payload 字段。
- [x] 增加 adapter 和 worker retry/idempotency 单元测试。
- [x] 新增 `docs/design-docs/transfer-gateway-dispatcher.md`。
- [x] 更新 `ARCHITECTURE.md`、`docs/design-docs/index.md` 和 `scripts/verify-static.sh`。
- [x] 完成完整验证命令并记录结果。
- [ ] 提交并推送 `feature/transfer-gateway-dispatcher`。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | Adapter 放在 `internal/transfer/gateway` | ownership 在 Transfer worker，依赖 Gateway delivery 契约，不进入 WebSocket internals。 |
| 2026-04-29 | offline recipient 映射为 `StatusSucceeded` 且不计入 delivered user | 当前没有 Redis 跨实例路由或离线推送；消息历史由 Message Service/PostgreSQL 权威保存，离线用户通过重连 pull 补偿。 |
| 2026-04-29 | no recipients 映射为 terminal failed | 这是事件契约错误，重试相同事件不会补出收件人，不能消耗 Gateway 调用或进入无限重试。 |
| 2026-04-29 | Gateway error 或 failed recipient 映射为 retryable | 本地连接写失败或 dispatcher 不可用可能通过重试、连接清理或后续 routing 恢复，由 Worker 生成 `RetryDecision`。 |

## 验证方式

计划执行：

- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl --version`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH bash -lc 'for f in api/*.api; do goctl api validate -api "$f"; done'`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH bash scripts/verify-static.sh`
- `docker compose config`

## 风险与回滚

- 风险：offline 被误认为 delivery ACK。缓解：文档和 adapter result 明确 offline 只代表本地 Gateway 未投递，send ACK 语义不变。
- 风险：未来真实 consumer 的 event payload 字段与 transfer event 不一致。缓解：新增字段为可选，adapter 只做浅转换，Kafka/outbox consumer 后续负责填充。
- 回滚：移除 `internal/transfer/gateway`、对应测试、transfer event 可选字段、文档和静态验证条目。

## 结果记录

已完成 Transfer Gateway Dispatcher adapter、单元测试、设计文档和静态验证条目。

验证结果：

- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH goctl --version`：`goctl version 1.10.1 linux/amd64`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH bash -c 'for f in api/*.api; do goctl api validate -api "$f"; done'`：5 个 API 文件均 `api format ok`
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...`：通过
- `PATH=/tmp/go/bin:$HOME/go/bin:$PATH bash scripts/verify-static.sh`：`static verification passed`
- `docker compose config`：通过
