# Agent Conversation Hosting

状态：Completed

## 背景

Agent/AI 回复必须作为普通 IM 消息进入 Message Service，不能直接写 `messages` 表或绕过 outbox、conversation seq、read state 和 Gateway 投递链路。第一阶段需要先冻结消息来源、Agent 元数据、托管配置、触发幂等和前端 AI 标识。

## 目标

- Message REST/RPC/model/storage 暴露 `message_origin=human|ai|system`。
- AI 消息记录 `agent_account_id`、`trigger_server_msg_id`、`agent_run_id` 和递归策略。
- Conversation hosting 能表示某会话由某个 Agent account 托管并启停。
- Agent trigger 使用现有 `internal/agentim` / `internal/agentruntime` seam，并通过 `MessageLogic.SendMessage` 写回。
- 同一 trigger 幂等，AI 消息默认不触发下一轮 AI。
- 前端消息气泡明确标注 AI/Agent 消息。

## 非目标

- 不接真实外部 LLM key。
- 不实现 shell/命令执行或未隔离 Python 执行。
- 不重构前端主框架。
- 不把 Agent 写回改成直接 repository/DB insert。

## 任务拆分

- [x] Task 1：补充 message origin/metadata 和 hosting 行为测试。
- [x] Task 2：扩展 Message API/Proto/Go model/repository/PostgreSQL migration/outbox/gateway 映射。
- [x] Task 3：新增 conversation hosting repository 与 Agent hosting handler seam，接入 `AgentRunOrchestrator`。
- [x] Task 4：更新前端 TS model/API 和 `MessagesPage` AI 标签 UI。
- [x] Task 5：更新产品/设计/架构/前端文档与静态检查。
- [x] Task 6：运行要求的验证命令，修复失败并记录结果。

## 决策日志

| 时间 | 决策 | 原因 |
| --- | --- | --- |
| 2026-05-02 | 托管触发先实现为消费已持久化消息事件的 service seam | 符合 IM/Agent 解耦，不改变用户消息持久化路径，也避免 Agent 直接写 DB。 |
| 2026-05-02 | AI 写回使用 `MessageLogic.SendMessage` 的扩展字段 | 复用 Message Service 的幂等、seq、outbox、read state 和验证。 |
| 2026-05-02 | Trigger 幂等键由 `event_id/server_msg_id + agent_account_id` 派生，并在 hosting 仓储中记录状态 | 防止重复事件产生重复 Agent 回复，同时允许失败状态显式暴露。 |
| 2026-05-02 | `MessageLogic` 增加 `MessageCreatedHook`，由 `ConversationHostingService` 消费 `message.created:<server_msg_id>` | 用户消息进入 Message Service 后即可触发托管 seam；AI 写回仍回到 Message Service，并由 origin/递归元数据防循环。 |

## 验证方式

```bash
export PATH=/tmp/go/bin:$HOME/go/bin:$PATH
goctl --version
for f in api/*.api; do goctl api validate -api "$f"; done
gofmt -w $(find . -name '*.go' -print)
go test ./...
npm --prefix web ci
npm run frontend:test
npm run frontend:build
npm run frontend:lint
bash scripts/verify-static.sh
POSTGRES_PASSWORD=local-postgres-placeholder REDIS_PASSWORD=local-redis-placeholder MINIO_ROOT_USER=local-minio-user MINIO_ROOT_PASSWORD=local-minio-password docker compose -f deploy/middleware/docker-compose.yml config >/tmp/agent_conversation_hosting_compose_config.txt
bash -n scripts/dev-up.sh
bash -n scripts/dev-demo-data.sh
git diff --check
```

## 风险与回滚

- Message schema 扩展影响 REST/RPC/front-end contracts；回滚需要同时回退 migration、models 和 generated proto。
- Hosting seam 若接入生产 runtime 缺配置，必须失败返回，不得 mock 成功。
- 若 Agent trigger 失败，状态必须可见；不得吞错或静默改为 demo 回复。

## 结果记录

- Message Service、REST/RPC/proto、PostgreSQL migration、outbox/Kafka/Gateway/Transfer payload 均暴露 `message_origin=human|ai|system` 和 AI metadata。
- 新增 `agent_conversation_hosting` 与 `agent_trigger_idempotency` 仓储，托管触发通过 `AgentRunOrchestrator -> MessageServiceResponseWriter -> MessageLogic.SendMessage` 写回。
- `MessageLogic.SetMessageCreatedHook` 将已持久化消息快照交给 hosting seam；AI/system 消息默认不触发 Agent，重复 trigger 由 idempotency key 去重。
- `MessagesPage` 在消息气泡和会话预览中显示 `AI/Agent`/系统标签，并保留真实 API adapter。
- 已同步更新架构、产品规格、前后端契约、前端约定、Agent hosting 设计文档和静态检查。

验证结果（2026-05-02）：

- `goctl --version` -> passed (`goctl version 1.10.1 linux/amd64`)。
- `for f in api/*.api; do goctl api validate -api "$f"; done` -> passed，6 个 API 均 `api format ok`。
- `gofmt -w $(find . -name '*.go' -print)` -> passed。
- `go test ./...` -> passed。
- `npm --prefix web ci` -> passed。
- `npm run frontend:test` -> passed，8 files / 18 tests。
- `npm run frontend:build` -> passed。
- `npm run frontend:lint` -> passed。
- `POSTGRES_PASSWORD=local-postgres-placeholder REDIS_PASSWORD=local-redis-placeholder MINIO_ROOT_USER=local-minio-user MINIO_ROOT_PASSWORD=local-minio-password docker compose -f deploy/middleware/docker-compose.yml config >/tmp/agent_conversation_hosting_compose_config.txt` -> passed。
- `bash -n scripts/dev-up.sh` -> passed。
- `bash -n scripts/dev-demo-data.sh` -> passed。
- `git diff --check` -> passed。
- `bash scripts/verify-static.sh` -> passed。
