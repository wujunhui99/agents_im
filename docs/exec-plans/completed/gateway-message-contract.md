# Gateway Message Contract

状态：Completed

归档日期：2026-05-01

## 背景

`feature/gateway-contract` 需要在 `message-service-contract` 并行开发时先稳定 Gateway 与 Message Service 的接口边界。该计划只落文档和纯 Go 契约映射，不实现真实 WebSocket server，不保存消息，不维护 seq/read state。

2026-05-01 状态对齐：该早期契约计划已被后续 `websocket-gateway`、`gateway-push-delivery`、`gateway-presence-routing` 和 `transfer-gateway-dispatcher` 计划覆盖，且这些提交已包含在当前 `main`。本文件从 active 归档到 completed；剩余 Gateway 生产问题不再由本早期契约计划跟踪，而由更具体的执行计划或后续新计划承接。

## 目标

- 新增 Gateway 到 Message Service 的设计契约文档。
- 新增客户端可感知的 Gateway message 产品规格。
- 新增 `internal/gateway` 纯契约代码，定义命令常量、请求/响应结构和映射函数。
- 新增测试验证 command 名称和映射字段，不依赖尚未合入的 message-service-contract 代码。
- 更新静态校验，要求 gateway-message 文档和测试存在。
- 为后续 gateway skeleton 实现列出可拆分任务。

## 非目标

- 不创建 `proto/message.proto` 或 `api/message.api`。
- 不实现真实 WebSocket server。
- 不实现 Message Service RPC client。
- 不持久化消息。
- 不生成 `server_msg_id` 或 conversation `seq`。
- 不维护 `has_read_seq`、`max_seq` 或 `unread_count`。
- 不实现 Kafka、push、离线补偿或在线 fanout。

## 任务拆分

- [x] Task 1：阅读 `AGENTS.md`、`ARCHITECTURE.md`、`docs/product-specs/message-chain.md`、`docs/design-docs/message-chain-contract.md`、`docs/design-docs/websocket-reliability.md`。
- [x] Task 2：新增 `docs/design-docs/gateway-message-contract.md`。
- [x] Task 3：新增 `docs/product-specs/gateway-message-contract.md`。
- [x] Task 4：创建本 active 执行计划。
- [x] Task 5：新增 `internal/gateway/contract.go` 纯契约映射代码。
- [x] Task 6：新增 `tests/gateway_contract_test.go`。
- [x] Task 7：更新 `scripts/verify-static.sh`，保留 user/auth/friends/groups/message 既有检查并新增 gateway-message 检查。
- [x] Task 8：运行 `PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)`。
- [x] Task 9：运行 `PATH=/tmp/go/bin:$PATH go test ./...`。
- [x] Task 10：运行 `bash scripts/verify-static.sh`。
- [x] Task 11：Evaluator 检查文档、代码、测试和边界一致性。
- [x] Task 12：`feature/gateway-contract` 历史分支已存在于远端并已合入当前 `main`；本次状态对齐不 push。

## 后续 gateway skeleton 实现任务状态对齐

| 原后续任务 | 当前承接状态 |
| --- | --- |
| WebSocket command envelope decoder/encoder，保持 `requestId` 透传 | 已由 `websocket-gateway` 实现，支持 canonical `request_id/type/payload` 和 frontend `requestId/command/payload` alias。 |
| 连接鉴权，注入连接用户，禁止客户端伪造 | 已由 `websocket-gateway` 实现；Gateway 使用 JWT claim 注入 `sender_id` / `user_id`。 |
| Message Service 调用边界 | 已由当前 Message API/RPC contract 和 `websocket-gateway` 承接；Gateway 在本仓库阶段直接调用 `MessageLogic`，不在该早期计划继续追踪独立 RPC client。 |
| Gateway command router | 已由 `websocket-gateway` 实现 `heartbeat`、`send_message`、`pull_messages`、`get_conversation_seqs`、`mark_conversation_read`。 |
| command ACK writer，区分 command ACK 与 delivery ACK | 已由 `websocket-gateway` 和 `gateway-push-delivery` 文档/测试覆盖；command ACK 不代表收件端收到或已读。 |
| 心跳检测和连接生命周期 | 已由 `websocket-gateway` 实现，后续由 `gateway-presence-routing` 接入 Presence TTL refresh。 |
| 连接 registry，支持按用户/设备投递 | 已由 `websocket-gateway` 的 `ConnectionManager` 和 `gateway-push-delivery` 的 in-process fanout 覆盖。 |
| `message.accepted` 在线投递入口 | 已由 `gateway-push-delivery` 和 `transfer-gateway-dispatcher` 覆盖为 `message_received` push；跨实例远程 Gateway 调用仍不是这些已归档计划的实现范围。 |
| `message.read` 在线通知和 read receipt push ACK | 未闭环，不在本早期 Gateway contract 中假关闭；继续由 active 的 `read-receipts.md` 跟踪。 |
| reconnect sync 集成测试 | 已由 `websocket-gateway` 的 reconnect sync flow 测试覆盖。 |
| 错误映射测试 | Gateway handshake、invalid command 和 frontend error envelope 已有测试覆盖；更完整业务错误映射随 Message/Gateway 后续专项继续补强。 |

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-04-29 | Gateway contract 不引用 `messagepb`。 | `message-service-contract` 并行开发，避免分支间大面积冲突和生成代码依赖。 |
| 2026-04-29 | 命令名使用 `send_message`、`pull_messages`、`get_conversation_seqs`、`mark_conversation_read`。 | 与 Message Service 四个 RPC 一一对应，并保持客户端命令语义清晰。 |
| 2026-04-29 | Gateway 只注入连接用户，不拥有 message/read state。 | 符合架构中长连接层职责和 Message Service 的权威数据边界。 |
| 2026-04-29 | Command ACK 与 delivery ACK 明确分离。 | 发送成功只代表 Message Service 已接受/存储，不代表收件人在线收到。 |

## Planner 结果

- 已确认当前 worktree：`/home/ws/project/worktrees/gateway-contract`。
- 已确认当前分支：`feature/gateway-contract`。
- 已阅读任务要求指定的五个文档。
- 已阅读 `docs/PLANS.md`、现有执行计划、Go 目录结构和 `scripts/verify-static.sh`。
- 已规划为文档契约、纯 Go 映射、测试、静态校验和验证提交五个部分。

## 验证方式

计划运行：

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)
PATH=/tmp/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
```

## 风险与回滚

- 风险：后续 `message-service-contract` 的字段命名可能微调。缓解：当前文档和测试按已存在 `message-chain-contract.md` 锁定，后续只需小范围调整映射。
- 风险：未来 Gateway 实现可能混淆 command ACK 与 delivery ACK。缓解：产品和设计文档均明确第一阶段 ACK 语义。
- 回滚：移除 gateway-message 文档、`internal/gateway`、gateway 测试，并撤销 `scripts/verify-static.sh` 的 gateway 检查。

## 结果记录

## 2026-05-01 状态对齐结果

- 当前 `main` 已包含 `feature/gateway-contract`、`feature/websocket-gateway`、`feature/gateway-push-delivery`、`feature/gateway-presence-routing` 和 `feature/transfer-gateway-dispatcher` 的相关提交。
- 该计划的早期契约产物仍有效：`docs/design-docs/gateway-message-contract.md`、`docs/product-specs/gateway-message-contract.md`、`internal/gateway/contract.go` 和 `tests/gateway_contract_test.go`。
- 原“后续 gateway skeleton 实现任务”大多已被更具体计划覆盖；仍未完整闭环的 Gateway 生产问题包括跨实例远程 Gateway transport、offline push、delivery ACK worker，以及 read receipt 的 `message.read` 通知和 push ACK。前几项由更具体 Gateway/Transfer/Delivery 计划或后续新计划跟踪，read receipt 剩余事项由 active 的 `read-receipts.md` 跟踪。
- 本次只做文档状态对齐，未启动真实依赖，也未声称端到端验证。

## Generator 结果

- 已新增设计文档 `docs/design-docs/gateway-message-contract.md`，定义四个 WebSocket command 到 Message Service RPC 的字段映射。
- 已新增产品规格 `docs/product-specs/gateway-message-contract.md`，定义连接后发送、拉取、已读、ACK 的第一阶段客户端语义。
- 已新增 `internal/gateway/contract.go`，包含 command 常量、请求/响应结构、字段映射表和纯映射函数。
- 已新增 `tests/gateway_contract_test.go`，验证 command 名称、RPC 方法名、请求字段映射、连接用户注入和响应字段透传。
- 已更新 design/product 索引，加入 gateway-message contract 文档。

## Evaluator 结果

验证命令：

```bash
PATH=/tmp/go/bin:$PATH gofmt -w $(find . -name "*.go" -print)
PATH=/tmp/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
```

验证结果：

```text
go test ./...:
ok  	github.com/wujunhui99/agents_im/tests	0.040s

scripts/verify-static.sh:
static verification passed
```

一致性检查：

- Gateway contract 未引用 `messagepb`、`proto/message.proto` 或 `api/message.api`，避免依赖并行分支。
- `send_message` 只把连接用户映射为 `sender_id`，不生成 `server_msg_id`、`conversation_id` 或 `seq`。
- `pull_messages`、`get_conversation_seqs`、`mark_conversation_read` 都只把连接用户映射为 `user_id`，不在 Gateway 内保存消息或 read state。
- Command ACK 与未来 delivery ACK 在产品和设计文档中已明确分离。
- `scripts/verify-static.sh` 只追加 gateway-message 检查，未删除 user/auth/friends/groups 既有检查。
