# Agent Conversation Hosting

状态：Implemented

## 背景

Agent 回复必须作为普通 IM 消息进入 Message Service，复用 `server_msg_id`、conversation `seq`、read state、outbox 和后续 Gateway/Transfer 投递链路。Agent 系统不能直接写 `messages` 表，也不能在缺少真实 runtime/provider 配置时伪造生产 LLM 成功。

## 目标

- Message Service 持久化并暴露 `message_origin=human|ai|system`。
- AI 消息记录 `agent_account_id`、`trigger_server_msg_id`、`agent_run_id` 和 `allow_recursive_trigger`。
- Conversation hosting 能表示某个 `conversation_id` 由哪个 `agent_account_id` 托管，且可启停。
- `message.created` 事件进入 Agent hosting seam 后，通过 `AgentRunOrchestrator -> MessageServiceResponseWriter -> MessageLogic.SendMessage` 写回 AI 消息。
- 同一 trigger 使用 idempotency key 防重复回复。
- AI 消息默认不再触发 AI，除非会话策略和消息元数据都显式允许递归。

## 非目标

- 不接入真实外部 LLM key。
- 不实现 shell/命令执行或未隔离 Python 执行。
- 不把 Agent response writer 接到 message repository 或 WebSocket 推送。
- 不实现完整异步 Webhook dispatcher；第一阶段提供可测试 service seam。

## 数据模型

`messages` 增加：

```text
message_origin: human | ai | system
agent_account_id
trigger_server_msg_id
agent_run_id
allow_recursive_trigger
```

规则：

- 普通用户发送默认 `human`。
- Agent 自动回复必须使用 `ai`，`agent_account_id` 必须等于 `sender_id`。
- `trigger_server_msg_id` 指向触发本次 Agent run 的已持久化消息。
- `system` 用于系统提示/状态消息，不携带 Agent metadata。

新增表：

```text
agent_conversation_hosting
- conversation_id primary key
- agent_account_id
- enabled
- allow_agent_message_recursion

agent_trigger_idempotency
- idempotency_key primary key
- conversation_id
- agent_account_id
- trigger_server_msg_id
- trigger_event_id
- status: running | succeeded | failed
- response_server_msg_id
- error_message
```

失败 trigger 会记录为 `failed`；相同 key 后续可重新进入 `running`。`running` 或 `succeeded` 的 key 不会重复执行。

## 数据流

```text
MessageLogic.SendMessage accepts human message
-> messages row includes message_origin=human
-> message.created outbox payload includes origin metadata
-> MessageCreatedHook calls Agent hosting seam with the persisted message snapshot
-> target Agent resolved from hosting config or explicit target list
-> AgentRunOrchestrator calls agentruntime.Runtime
-> MessageServiceResponseWriter calls MessageLogic.SendMessage
-> Message Service persists AI message with message_origin=ai
-> message_outbox records normal message.created event for AI message
```

`internal/logic.MessageLogic` exposes a narrow `MessageCreatedHook`; the hook runs after message persistence and idempotency resolution, using a stable `message.created:<server_msg_id>` event id. `internal/agentim.ConversationHostingService` implements that hook, owns trigger selection and idempotency, and does not own message storage. `MessageServiceResponseWriter` is the only writeback path and depends on the narrow `MessageSender` interface compatible with `MessageLogic.SendMessage`.

## 触发规则

- Hosted conversation：`agent_conversation_hosting.enabled=true` 时，目标 Agent 为配置的 `agent_account_id`。
- Private Agent chat：hosting seam 可通过 `AgentAccountResolver` 校验 receiver 是 active Agent account。
- Group trigger：第一阶段由 explicit `TargetAgentAccountIDs` 或 hosting config 提供目标 Agent；完整 @ 解析留给上游事件构造方。
- AI-origin message：默认跳过。只有 `agent_conversation_hosting.allow_agent_message_recursion=true` 且消息 `allow_recursive_trigger=true` 时才允许递归。
- System-origin message：不触发 Agent。

## 失败优先

- 未配置 repository、runner、Message Service writer 或 runtime builder 时返回明确错误。
- Runtime/build/audit/writeback 失败时返回错误，并把 trigger idempotency 状态记录为 `failed`。
- 缺少真实生产 provider 配置时 runtime 必须 fail closed；测试可注入 deterministic runtime。

## 验证方式

```bash
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./internal/agentim ./internal/repository ./tests
PATH=/tmp/go/bin:$HOME/go/bin:$PATH go test ./...
bash scripts/verify-static.sh
```
