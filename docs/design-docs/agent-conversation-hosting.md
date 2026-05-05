# Agent Conversation Hosting

状态：Implemented / Issue #3 V1 extended

## 背景

Agent 回复必须作为普通 IM 消息进入 Message Service，复用 `server_msg_id`、conversation `seq`、read state、outbox 和后续 Gateway/Transfer 投递链路。Agent 系统不能直接写 `messages` 表，也不能在缺少真实 runtime/provider 配置时伪造生产 LLM 成功。

## 目标

- Message Service 持久化并暴露 `message_origin=human|ai|system`。
- AI 消息记录 `agent_account_id`、`trigger_server_msg_id`、`agent_run_id` 和 `allow_recursive_trigger`。
- Conversation hosting 能表示某个 `conversation_id` 由哪个 `agent_account_id` 托管，且可启停。
- Issue #3 AI Hosting V1 支持普通用户在双人单聊中开启“由 AI 代我回复”，设置作用域为 `owner_account_id + conversation_id`，默认关闭。
- 同一个双人单聊最多只能有一方开启 AI 托管；一方开启后，另一方读取状态时必须看到不可用原因，更新开启时必须得到冲突错误。
- `message.created` 事件进入 Agent hosting seam 后，通过 `AgentRunOrchestrator -> MessageServiceResponseWriter -> MessageLogic.SendMessage` 写回 AI 消息。
- 同一 trigger 使用 idempotency key 防重复回复。
- AI 消息默认不再触发 AI，除非会话策略和消息元数据都显式允许递归。

## 非目标

- 不在缺少真实外部 LLM/provider 配置时伪造成功。
- 不实现 shell/命令执行或未隔离 Python 执行。
- 不实现完整异步 Webhook dispatcher；第一阶段提供可测试 service seam。
- Issue #3 V1 不支持群聊托管。

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

conversation_ai_hosting_settings
- owner_account_id
- conversation_id
- enabled
- mode: auto_reply
- max_recent_messages
- summary_enabled
- created_at
- updated_at
```

失败 trigger 会记录为 `failed`；相同 key 后续可重新进入 `running`。`running` 或 `succeeded` 的 key 不会重复执行。

`conversation_ai_hosting_settings` 使用 `(owner_account_id, conversation_id)` 主键保存当前用户设置，并通过 `enabled=true` 的 `conversation_id` 唯一约束保证同一双人单聊只有一方可开启。V1 只允许 `single:*` 会话，默认 `max_recent_messages=30`，`summary_enabled=false` 作为后续 rolling summary 占位。

## 数据流

```text
MessageLogic.SendMessage accepts human message
-> messages row includes message_origin=human
-> message.created outbox payload includes origin metadata
-> MessageCreatedHook calls Agent hosting seam with the persisted message snapshot
-> target Agent resolved from direct-chat AI hosting setting, hosting config, or explicit target list
-> AgentRunOrchestrator calls agentruntime.Runtime
-> MessageServiceResponseWriter calls MessageLogic.SendMessage
-> Message Service persists AI message with message_origin=ai
-> message_outbox records normal message.created event for AI message
```

`internal/logic.MessageLogic` exposes a narrow `MessageCreatedHook`; the hook runs after message persistence and idempotency resolution, using a stable `message.created:<server_msg_id>` event id. `internal/agentim.ConversationHostingService` implements that hook, owns trigger selection and idempotency, and does not own message storage. `MessageServiceResponseWriter` is the only writeback path and depends on the narrow `MessageSender` interface compatible with `MessageLogic.SendMessage`.

## 触发规则

- Issue #3 direct-chat AI hosting：当 `conversation_ai_hosting_settings.enabled=true`，且触发消息是对端发送的 `human` 单聊消息时，目标 owner 为开启托管的一方。AI 回复必须通过 `MessageLogic.SendMessage` 写入，`message_origin=ai`，`sender_id/agent_account_id` 均为被托管 owner，`trigger_server_msg_id` 指向触发消息。
- Hosted conversation：`agent_conversation_hosting.enabled=true` 时，目标 Agent 为配置的 `agent_account_id`。
- Private Agent chat：hosting seam 可通过 `AgentAccountResolver` 校验 receiver 是 active Agent account。
- Group trigger：第一阶段由 explicit `TargetAgentAccountIDs` 或 hosting config 提供目标 Agent；完整 @ 解析留给上游事件构造方。
- AI-origin message：默认跳过。只有 `agent_conversation_hosting.allow_agent_message_recursion=true` 且消息 `allow_recursive_trigger=true` 时才允许递归。
- System-origin message：不触发 Agent。
- Issue #3 V1 不处理群聊；群聊读取/更新 AI 托管设置返回显式错误。

## HTTP API

```text
GET /conversations/:conversation_id/ai-hosting
PUT /conversations/:conversation_id/ai-hosting
```

`PUT` body:

```json
{
  "enabled": true
}
```

Response data:

```json
{
  "conversationId": "single:1001:2002",
  "chatType": "single",
  "enabled": false,
  "available": false,
  "peerEnabled": true,
  "unavailableReason": "对方已开启 AI 托管，本会话暂时只能由一方开启",
  "maxRecentMessages": 30,
  "summaryEnabled": false
}
```

未开启时 `enabled=false` 且 `available=true`。当前用户已开启时 `enabled=true` 且 `available=true`。对端已开启时 `enabled=false`、`available=false`、`peerEnabled=true`，前端禁用当前用户的开关并展示中文原因。

## Runtime Context

托管回复 runtime request 只能包含 bounded recent messages，当前实现最多取最近 30 条，并携带 `summary_used=false` 占位；不能把全量历史、token、prompt 私密内容或 provider secret 写入日志或响应。缺少 DeepSeek provider/model 配置时 production runtime fail closed，触发记录为失败，不创建硬编码或 fake AI 消息。

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
