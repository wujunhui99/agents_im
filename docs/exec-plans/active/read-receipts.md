# read-receipts

状态：Active

## 背景

消息链路契约已经定义 `user_id + conversation_id -> has_read_seq` 作为第一阶段会话级已读状态。该计划用于把已读能力拆成可并行任务，避免与 message-service-contract 分支在 handler、proto、api 和 Gateway 上产生大范围冲突。

## 目标

- 固化 read receipts 产品规格和技术设计。
- 提供不依赖 message service handler 的纯函数基础能力。
- 明确后续 read state repository、notification、Gateway ACK 的边界和验收方式。

## 非目标

- 不实现完整 message handler。
- 不保存消息。
- 不直接修改 Gateway。
- 不引入 message-service-contract 分支尚未存在的类型。

## 任务拆分

- [x] Task 1：新增 read receipts 产品规格，定义客户端标记已读、未读数、重复请求、回退请求、越界请求行为。
- [x] Task 2：新增 read receipts 设计文档，定义状态模型、单调推进、sender/receiver 语义和未来群聊扩展点。
- [x] Task 3：新增纯函数和单元测试，覆盖未读数、单调推进、幂等、回退和越界拒绝。
- [ ] Task 4：实现 read state repository 接口，提供 `GetUserHasReadSeq`、`SetUserHasReadSeqMax`、`GetConversationMaxSeq`，并用 max 更新保证并发单调性。
- [ ] Task 5：实现 `message.read` notification plumbing，仅在 read cursor 实际推进时发事件，重复和回退请求不发事件。
- [ ] Task 6：定义 Gateway mark-read command 和 read receipt push ACK，ACK 只确认事件投递，不改变 read state。
- [ ] Task 7：接入 message service contract 后补齐 HTTP/RPC handler 集成测试。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | 以 `user_id + conversation_id -> has_read_seq` 作为第一阶段唯一权威读状态 | 与消息链路契约一致，便于 repository、Gateway 和 notification 并行实现 |
| 2026-04-29 | 越界 `has_read_seq > max_seq` 必须拒绝，不做 clamp | 避免客户端隐藏尚未同步的消息 |
| 2026-04-29 | 本分支只新增文档、纯函数和测试，不实现 Gateway 或完整 message handler | 减少与 message-service-contract 分支冲突 |

## 验证方式

- `PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)`
- `PATH=/tmp/go/bin:$PATH go test ./...`
- `bash scripts/verify-static.sh`

## 风险与回滚

- 风险：未来 repository 接入时如果使用普通赋值，可能让旧设备请求回退 `has_read_seq`。
- 缓解：repository 必须使用 max 更新或等价事务保护，并保留单调推进测试。
- 回滚：本分支只增加文档、纯函数和测试，可直接回滚新增文件及 `scripts/verify-static.sh` 的 read receipts 检查。

## 结果记录

- 当前分支完成 read receipts contract、纯函数基础能力和静态校验入口。
- 后续 repository、notification、Gateway ACK 任务仍保持 Active。
