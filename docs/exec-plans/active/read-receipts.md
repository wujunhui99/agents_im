# read-receipts

状态：Active

## 背景

消息链路契约已经定义 `user_id + conversation_id -> has_read_seq` 作为第一阶段会话级已读状态。该计划用于把已读能力拆成可并行任务，避免与 message-service-contract 分支在 handler、proto、api 和 Gateway 上产生大范围冲突。

2026-05-01 状态对齐：当前 `main` 已有 Message Service repository/logic、HTTP/RPC `MarkConversationAsRead`、Gateway `mark_conversation_read` command ACK 和相关 static/unit/frontend contract 验证。`message.read` notification plumbing、read receipt server push 和客户端 push ACK 仍未完整闭环，因此本计划继续保持 Active。

## 目标

- 固化 read receipts 产品规格和技术设计。
- 记录当前已落地的 read state repository、HTTP/RPC mark-read、Gateway mark-read command 基础能力。
- 明确剩余 `message.read` notification、Gateway read receipt push 和 push ACK 的边界和验收方式。

## 非目标

- 本次状态对齐不修改业务代码。
- 不新增 `message.read` outbox/Kafka/notification plumbing。
- 不实现 read receipt server push、客户端 push ACK、offline push 或真实端到端联调。
- 不声称真实 E2E；只能记录 static/unit/frontend contract verification 能证明的状态。

## 任务拆分

- [x] Task 1：新增 read receipts 产品规格，定义客户端标记已读、未读数、重复请求、回退请求、越界请求行为。
- [x] Task 2：新增 read receipts 设计文档，定义状态模型、单调推进、sender/receiver 语义和未来群聊扩展点。
- [x] Task 3：新增纯函数和单元测试，覆盖未读数、单调推进、幂等、回退和越界拒绝。
- [x] Task 4：实现 read state repository 基础能力。当前 `MessageRepository` 提供 `GetConversationSeqStates` 和 `SetUserHasReadSeqMax`；memory/PostgreSQL 实现使用单调 max 更新并拒绝 `has_read_seq > max_seq`。未单独暴露 `GetUserHasReadSeq` / `GetConversationMaxSeq`，其语义由 `GetConversationSeqStates` 返回的 `has_read_seq` / `max_seq` 覆盖。
- [x] Task 5：接入 Message Service HTTP/RPC mark-read 基础路径，`POST /conversations/:conversation_id/read` 和 RPC `MarkConversationAsRead` 调用同一 `MessageLogic.MarkConversationAsRead`。
- [x] Task 6：接入 Gateway `mark_conversation_read` command 和 command ACK。该 ACK 只代表 mark-read command 已由 Message Service 处理，不是 read receipt push ACK。
- [x] Task 7：接入 message service contract 后补齐 HTTP/RPC/Gateway mark-read 的 static/unit/frontend contract 测试覆盖。
- [ ] Task 8：实现 `message.read` notification plumbing，仅在 read cursor 实际推进时发事件，重复和回退请求不发事件；事件需要进入后续 outbox/Kafka/notification 链路或等价可靠通道。
- [ ] Task 9：实现 Gateway read receipt server push 和客户端 push ACK；ACK 只确认 read receipt event 投递，不改变 Message Service read state。
- [ ] Task 10：补齐真实依赖下的端到端验收记录；未启动 PostgreSQL/Redis/Kafka/Gateway 多进程并实际请求前，不得声称 E2E 完成。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 以 `user_id + conversation_id -> has_read_seq` 作为第一阶段唯一权威读状态 | 与消息链路契约一致，便于 repository、Gateway 和 notification 并行实现 |
| 2026-04-29 | 越界 `has_read_seq > max_seq` 必须拒绝，不做 clamp | 避免客户端隐藏尚未同步的消息 |
| 2026-04-29 | 本分支只新增文档、纯函数和测试，不实现 Gateway 或完整 message handler | 减少与 message-service-contract 分支冲突 |
| 2026-05-01 | repository、HTTP/RPC mark-read 和 Gateway mark-read command 已由后续 main 代码补齐，但 `message.read` notification 与 read receipt push ACK 不关闭 | 防止 active 计划继续误导，同时避免把未闭环的通知/ACK 假关闭 |

## 验证方式

- `PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)`
- `PATH=/tmp/go/bin:$PATH go test ./...`
- `bash scripts/verify-static.sh`

## 风险与回滚

- 风险：未来 repository 接入时如果使用普通赋值，可能让旧设备请求回退 `has_read_seq`。
- 缓解：repository 必须使用 max 更新或等价事务保护，并保留单调推进测试。
- 回滚：本分支只增加文档、纯函数和测试，可直接回滚新增文件及 `scripts/verify-static.sh` 的 read receipts 检查。

## 结果记录

- 已完成 read receipts contract、纯函数基础能力和静态校验入口。
- 当前 `main` 已完成基础 read state mutation：`internal/repository.MessageRepository`、memory/PostgreSQL `SetUserHasReadSeqMax`、`MessageLogic.MarkConversationAsRead`、HTTP `POST /conversations/:conversation_id/read`、RPC `MarkConversationAsRead`、Gateway `mark_conversation_read` command ACK。
- 已有验证覆盖包括 `tests/read_receipts_test.go`、`tests/message_service_test.go`、`tests/postgres_persistence_integration_test.go`、`tests/websocket_gateway_test.go` 和 `tests/mvp_backend_test.go` 中的 static/unit/frontend contract 场景；本次状态对齐未运行真实 E2E。
- 仍保持 Active 的剩余事项：`message.read` event 只存在于 messaging schema/contract 中，尚未由 mark-read 成功推进时可靠发出；Gateway 尚无 read receipt server push 和客户端 push ACK 闭环；Kafka transfer consumer 当前仍明确不消费 `message.read`。
