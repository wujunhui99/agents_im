# Message Storage Implementation Plan

状态：Active

## 背景

消息链路第一阶段需要稳定的存储契约，以便 message service、gateway、read receipts 和 PostgreSQL repository 可以并行实现。当前分支只定义 storage 独立契约，不实现 HTTP handler，不修改 message service API 主契约。

## 目标

- 固化消息存储的产品保证、技术设计和后续执行任务。
- 定义 PostgreSQL repository 需要实现的接口语义。
- 为幂等、会话内 seq、消息拉取和已读状态单调性预留测试任务。

## 非目标

- 不创建或修改 `api/message.api`、`proto/message.proto`。
- 不实现 message HTTP/RPC handler。
- 不接入 Kafka、Gateway、Push 或 WebSocket ACK。
- 不引入真实 PostgreSQL/Redis 依赖到当前代码。

## 任务拆分

- [x] Task 1：创建 `docs/design-docs/message-storage.md`，定义 PostgreSQL/Redis 存储模型、唯一约束、seq 分配和事务边界。
- [x] Task 2：创建 `docs/product-specs/message-storage.md`，定义幂等、顺序、可拉取和已读状态单调的产品保证。
- [x] Task 3：新增独立 Go storage contract 文件，仅包含接口、错误类型和纯 helper。
- [x] Task 4：更新静态校验，要求 message storage 文档存在。
- [ ] Task 5：实现 PostgreSQL migration，创建 `messages`、`conversation_threads`、`user_conversation_states`、`message_idempotency_keys`。
- [ ] Task 6：实现 PostgreSQL repository 的 `CreateMessageIdempotent` 事务，覆盖幂等冲突和并发 seq 分配。
- [ ] Task 7：实现 `PullMessages`，支持 conversation seq 范围、limit、asc/desc 和空结果。
- [ ] Task 8：实现 `GetConversationSeqStates`，返回 `max_seq`、`has_read_seq`、`unread_count`、`last_message`。
- [ ] Task 9：实现 `SetUserHasReadSeqMax`，保证 `has_read_seq` 不回退并拒绝超过 `max_seq` 的请求。
- [ ] Task 10：补充 repository 单元/集成测试，覆盖重复发送、冲突、并发发送、拉取、已读单调和 seq 越界。
- [ ] Task 11：评估 Redis cache/outbox 引入点，确认 Redis 只作为可重建缓存或短期幂等加速。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | PostgreSQL 是消息历史、会话 max seq 和用户 read state 的权威存储 | 保证持久化语义清晰，Redis 故障不会破坏历史或已读状态 |
| 2026-04-29 | 每个 conversation 通过 `conversation_threads` 行锁分配 seq | Phase 1 优先保证会话内连续、唯一、有序 |
| 2026-04-29 | `sender_id + client_msg_id` 是幂等唯一键 | 与 message-chain 产品规格一致，支持客户端安全重试 |
| 2026-04-29 | 当前分支不实现 message service API/proto/handler | 避免与 `feature/message-service-contract` 大量冲突 |

## 验证方式

当前契约阶段：

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)
PATH=/tmp/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
```

后续 PostgreSQL repository 阶段：

- 使用事务级测试验证重复发送不创建重复消息。
- 使用并发测试验证单 conversation seq 唯一且连续。
- 使用 read-state 测试验证低 seq 不会覆盖高 seq。
- 使用 pull 测试验证空范围、limit 和排序行为。

## 风险与回滚

- 风险：行锁在超大群高并发发送时会成为热点。回滚/缓解方式是在保持当前接口不变的前提下引入分区队列或异步 seq allocator。
- 风险：Redis 缓存污染可能导致读到过期状态。回滚/缓解方式是禁用 Redis 读取并回落 PostgreSQL。
- 风险：未来 message-service-contract 合并时出现类型命名差异。缓解方式是当前 Go 文件使用 storage 前缀类型，不依赖未合入的 proto 或 handler 类型。

## 结果记录

- 2026-04-29：创建 storage 产品规格、设计文档、执行计划和独立 repository contract。
