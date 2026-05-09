# Issue 46 AI Hosting Async Replies

状态：Completed

## 背景

AI 托管当前在 `MessageLogic.SendMessage -> MessageCreatedHook -> ConversationHostingService -> AgentRunOrchestrator` 路径中同步等待 runtime。慢 LLM 或 provider 失败会影响原始人类消息发送体验。

## 目标

- 人类消息先完成 Message Service 的存储、seq、read state、outbox 和返回。
- 托管 owner 的 read seq 在 trigger 被接受后立即推进到 trigger seq，独立于 AI 生成。
- AI 托管生成在后台执行，完成后仍通过 `MessageServiceResponseWriter -> MessageLogic.SendMessage` 写入 `message_origin=ai` 消息。
- 同一 trigger 使用现有 `agent_trigger_idempotency` 幂等记录避免重复回复；失败记录为 failed，不生成假回复。

## 非目标

- 不扩展群聊 AI 托管。
- 不改变同一单聊只能一方开启托管的互斥规则。
- 不绕过 Message Service 直接写 `messages` 或推 WebSocket。
- 不引入 fake production LLM 成功。

## 任务拆分

- [x] 写失败测试覆盖慢 runtime 不阻塞 send、mark-read 先于生成完成、生成完成后入库、幂等、loop prevention、provider failure 不阻塞原始 send。
- [x] 将 hosting 触发拆成快速接受和后台执行：选择 target、写入 idempotency running、mark read、异步运行 runner。
- [x] 调整 MessageLogic hook 语义，确保 post-persist hook/runtime failure 不把已存储的人类 send 变成失败 ACK。
- [x] 更新 production wiring，为 hosting service 注入 read marker。
- [x] 更新设计文档和本计划结果记录。
- [x] 运行 focused tests、要求的 Go tests、static verification、diff check。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-09 | 第一版使用现有 `agent_trigger_idempotency` 作为接受/幂等状态，后台执行由 message-api 进程内异步 runner 承担 | 当前仓库没有可恢复 Agent trigger worker；该方案最小化改动，同时保证 send path 不等待 LLM、失败可记录、重复 trigger 不产生重复回复。 |
| 2026-05-09 | mark-read 通过 Message Repository 的 `SetUserHasReadSeqMax` seam 推进 hosted owner read seq | 这是现有 read-state 权威路径，保持 monotonic read seq，不直接改表。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
go test ./internal/agentim -run 'TestConversationAIHosting|TestConversationHosting' -count=1
go test ./internal/agentim ./internal/agentruntime/... ./internal/repository ./internal/outboxpublisher ./internal/transfer ./internal/logic ./tests
bash scripts/verify-static.sh
git diff --check
```

## 风险与回滚

- 风险：进程退出时正在运行的后台 generation 可能中断；幂等记录会阻止重复 running trigger，但完整可恢复 worker 是后续可靠性增强。
- 回滚：revert 本 issue commit 可恢复同步托管行为。

## 结果记录

- Root cause：AI hosting 的 `MessageCreatedHook` 在人类消息持久化后同步执行 runtime/writeback；慢 LLM 会拖住 send response，provider failure 还会让已入库的人类消息表现为发送失败。
- Fix：`ConversationHostingService` 改为快速接受 trigger：写入 `agent_trigger_idempotency` running、通过 `MessageRepositoryReadMarker` 推进 hosted owner read seq，然后在后台运行 `AgentRunOrchestrator`；AI 回复仍通过 `MessageServiceResponseWriter -> MessageLogic.SendMessage` 入库/outbox。`MessageLogic` 对 post-persist hook error 改为日志记录，不再把已接受消息返回失败。
- Frontend：未发现等待 AI 回复的专用阻塞交互；现有 composer 只等待 send API，而后端 send API 已不等待 AI generation。
- Limitation：当前后台执行为 message-api 进程内 async runner，idempotency 状态可防重复，但进程重启恢复 pending/running trigger 仍需后续 durable worker。
- Verification：
  - `go test ./internal/agentim ./internal/agentruntime/... ./internal/repository ./internal/outboxpublisher ./internal/transfer ./internal/logic ./tests`
  - `go test ./...`
  - `bash scripts/verify-static.sh`
  - `git diff --check`
