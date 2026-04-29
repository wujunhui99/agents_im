# IM-Agent API / Event Contract

状态：Accepted for v0.1

本文档固化 IM 后端与 Agent 系统之间第一阶段并行开发所需的最小接口契约。目标是让 IM 后端和 Agent 系统可以在不互相等待内部实现的情况下并行开发。

## OpenIM Webhook 参考

本契约参考 OpenIM 的 webhook 设计，但不完全照搬。

OpenIM 参考点：

- 配置文件：`docs/references/open-im-server/config/webhooks.yml`
- 消息回调结构：`docs/references/open-im-server/pkg/callbackstruct/message.go`
- 通用回调结构：`docs/references/open-im-server/pkg/callbackstruct/common.go`
- 回调命令常量：`docs/references/open-im-server/pkg/callbackstruct/constant.go`
- Webhook HTTP 客户端：`docs/references/open-im-server/pkg/common/webhook/http_client.go`

OpenIM 的关键设计：

- webhook 基础 URL 统一配置，例如 `url: http://127.0.0.1:10006/callbackExample`。
- 具体事件通过 `/<callbackCommand>` 拼接为最终 URL。
- 支持 before/after 两类回调，例如 `beforeSendSingleMsg`、`afterSendSingleMsg`、`afterSendGroupMsg`、`afterMsgSaveDB`。
- 每类回调可配置 `enable`、`timeout`、`failedContinue`、`attentionIds`、`deniedTypes`。
- 请求体包含 `callbackCommand`、`operationID`、`sendID`、`serverMsgID`、`clientMsgID`、`contentType`、`content`、`seq`、`atUserList` 等字段。
- 响应体使用 `actionCode`、`errCode`、`errMsg`、`errDlt`、`nextCode` 表达处理结果。

本项目采用相同思想：**统一 webhook base URL + event command path + 标准请求/响应结构 + timeout + failedContinue**。

## 设计目标

- 支持 IM 后端与 Agent 系统异步解耦。
- 支持 Agent 单聊与群聊。
- 支持 @Agent 触发、多 Agent 识别和工具结果写回。
- 支持幂等、重试、签名鉴权和链路追踪。
- 第一阶段只定义最小必要契约，避免过度设计。

## 通信方向

```text
IM Backend --Webhook/Event--> Agent System
Agent System --Internal API--> IM Backend
```

第一阶段包含两类接口：

1. IM -> Agent：Webhook 事件通知。
2. Agent -> IM：Agent 消息写回与上下文读取 API。

## 命名约定

### 外部事件名

使用小写点分格式：

```text
message.created
conversation.member_added
conversation.member_removed
agent.conversation_bound
```

### Webhook command

为了兼容 OpenIM 风格，HTTP path 使用 command 格式：

```text
callbackAfterSendSingleMsgCommand
callbackAfterSendGroupMsgCommand
callbackAfterMsgSaveDBCommand
callbackAfterConversationMemberChangedCommand
```

事件体中同时保留：

- `callback_command`：OpenIM 风格命令。
- `event_type`：本项目内部标准事件名。

## IM -> Agent Webhook

### Endpoint 约定

Agent 系统提供统一 webhook base URL：

```text
POST {AGENT_WEBHOOK_BASE_URL}/{callback_command}
```

示例：

```text
POST http://agent-service:8000/webhooks/im/callbackAfterSendSingleMsgCommand
POST http://agent-service:8000/webhooks/im/callbackAfterSendGroupMsgCommand
```

### 必需 Header

```http
Content-Type: application/json
X-Operation-ID: <operation_id>
X-Trace-ID: <trace_id>
X-Event-ID: <event_id>
X-IM-Timestamp: <unix_seconds>
X-IM-Signature: sha256=<hmac_sha256(timestamp + "." + raw_body)>
```

说明：

- `X-Operation-ID` 对齐 OpenIM `operationID`，用于单次业务操作追踪。
- `X-Trace-ID` 用于跨 IM、Agent、工具调用的链路追踪。
- `X-Event-ID` 用于幂等处理。
- `X-IM-Timestamp` 用于防重放。
- `X-IM-Signature` 使用共享密钥计算 HMAC-SHA256。

### 通用请求结构

```json
{
  "event_id": "evt_01H...",
  "event_type": "message.created",
  "callback_command": "callbackAfterSendGroupMsgCommand",
  "operation_id": "op_01H...",
  "trace_id": "trace_01H...",
  "occurred_at": "2026-04-28T00:00:00Z",
  "source": "im-backend",
  "payload": {}
}
```

### 通用响应结构

参考 OpenIM `CommonCallbackResp`，但字段命名使用 snake_case。

```json
{
  "action_code": 0,
  "err_code": 0,
  "err_msg": "",
  "err_detail": "",
  "next_code": 0,
  "retry_after_seconds": 0
}
```

字段说明：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `action_code` | int | `0` 表示成功；非 0 表示失败。 |
| `err_code` | int | 业务错误码。 |
| `err_msg` | string | 错误摘要。 |
| `err_detail` | string | 错误详情。 |
| `next_code` | int | 是否继续后续流程。第一阶段只使用 `0`。 |
| `retry_after_seconds` | int | 建议 IM 重试等待时间，`0` 表示使用默认策略。 |

### Webhook 超时与失败策略

配置参考 OpenIM：

```yaml
agentWebhook:
  url: http://agent-service:8000/webhooks/im
  timeout: 5
  failedContinue: true
  maxRetry: 3
  retryBackoff: 1s
```

第一阶段策略：

- timeout 默认 5 秒。
- IM -> Agent webhook 失败时不阻塞用户消息入库。
- `failedContinue=true`，IM 主链路继续。
- 失败事件进入重试队列或失败事件表。
- Agent 系统必须按 `event_id` 做幂等。

## Event 1：单聊消息发送后

### Command

```text
callbackAfterSendSingleMsgCommand
```

### Event Type

```text
message.created
```

### 触发时机

用户单聊消息写入 IM 后触发。若接收方是 Agent，或消息上下文需要 Agent 处理，则投递给 Agent 系统。

### Payload

```json
{
  "conversation_id": "single_user_123_agent_456",
  "conversation_type": "single",
  "message": {
    "message_id": "msg_123",
    "client_message_id": "client_msg_123",
    "seq": 1001,
    "send_id": "user_123",
    "recv_id": "agent_456",
    "group_id": "",
    "sender": {
      "type": "user",
      "id": "user_123",
      "nickname": "Junhui",
      "face_url": ""
    },
    "content_type": "text",
    "content": {
      "type": "text",
      "text": "你好，帮我分析一下这个问题"
    },
    "raw_content": "{\"text\":\"你好，帮我分析一下这个问题\"}",
    "send_time": 1777392000000,
    "create_time": 1777392000000,
    "at_user_ids": [],
    "metadata": {}
  },
  "agent_context": {
    "target_agent_ids": ["agent_456"],
    "trigger": "direct_chat"
  }
}
```

## Event 2：群聊消息发送后

### Command

```text
callbackAfterSendGroupMsgCommand
```

### Event Type

```text
message.created
```

### 触发时机

用户群聊消息写入 IM 后触发。若消息 @Agent，或群配置允许 Agent 自动响应，则投递给 Agent 系统。

### Payload

```json
{
  "conversation_id": "group_789",
  "conversation_type": "group",
  "message": {
    "message_id": "msg_456",
    "client_message_id": "client_msg_456",
    "seq": 2033,
    "send_id": "user_123",
    "recv_id": "",
    "group_id": "group_789",
    "sender": {
      "type": "user",
      "id": "user_123",
      "nickname": "Junhui",
      "face_url": ""
    },
    "content_type": "text",
    "content": {
      "type": "text",
      "text": "@agent_456 总结一下今天的讨论"
    },
    "raw_content": "{\"text\":\"@agent_456 总结一下今天的讨论\"}",
    "send_time": 1777392000000,
    "create_time": 1777392000000,
    "at_user_ids": ["agent_456"],
    "metadata": {}
  },
  "agent_context": {
    "target_agent_ids": ["agent_456"],
    "trigger": "mention"
  }
}
```

## Event 3：消息入库后

### Command

```text
callbackAfterMsgSaveDBCommand
```

### Event Type

```text
message.saved
```

### 触发时机

消息成功持久化后触发。第一阶段可选，主要用于 Agent 系统构建长期上下文或异步索引。

### Payload

```json
{
  "conversation_id": "group_789",
  "conversation_type": "group",
  "message_id": "msg_456",
  "seq": 2033,
  "send_id": "user_123",
  "recv_id": "",
  "group_id": "group_789",
  "content_type": "text",
  "created_at": "2026-04-28T00:00:00Z"
}
```

## Event 4：会话成员变更

### Command

```text
callbackAfterConversationMemberChangedCommand
```

### Event Type

```text
conversation.member_changed
```

### 触发时机

Agent 被加入或移出会话、群成员变化、Agent 绑定关系变化时触发。

### Payload

```json
{
  "conversation_id": "group_789",
  "conversation_type": "group",
  "change_type": "member_added",
  "members": [
    {
      "member_type": "agent",
      "member_id": "agent_456",
      "role": "member"
    }
  ],
  "operator_id": "user_123"
}
```

## Agent -> IM API

Agent 系统通过 IM 后端内部 API 写回消息和查询上下文。第一阶段建议使用 HTTP REST，后续可切换或补充 gRPC。

### 通用 Header

```http
Content-Type: application/json
Authorization: Bearer <internal_service_token>
X-Operation-ID: <operation_id>
X-Trace-ID: <trace_id>
X-Request-ID: <request_id>
```

## API 1：Agent 写回消息

```text
POST /internal/agent/messages
```

### Request

```json
{
  "request_id": "req_123",
  "operation_id": "op_123",
  "trace_id": "trace_123",
  "conversation_id": "group_789",
  "conversation_type": "group",
  "agent_id": "agent_456",
  "reply_to_message_id": "msg_456",
  "content": {
    "type": "text",
    "text": "这是今天讨论的总结。"
  },
  "message_options": {
    "persist": true,
    "push": true,
    "need_ack": true
  },
  "metadata": {
    "tool_calls": [],
    "agent_run_id": "run_123"
  }
}
```

### Response

```json
{
  "message_id": "msg_agent_789",
  "seq": 2034,
  "server_time": "2026-04-28T00:00:01Z"
}
```

### 规则

- `request_id` 必须幂等。
- `agent_id` 必须是该会话成员，或具备向该会话发消息的权限。
- 写回消息必须进入 IM 标准消息链路。
- IM 后端负责持久化、分发、ACK 和离线补偿。

## API 2：写回 Agent 处理状态

```text
POST /internal/agent/status
```

### Request

```json
{
  "request_id": "req_status_123",
  "operation_id": "op_123",
  "trace_id": "trace_123",
  "conversation_id": "group_789",
  "agent_id": "agent_456",
  "reply_to_message_id": "msg_456",
  "status": "processing",
  "text": "Agent 正在分析...",
  "ttl_seconds": 30
}
```

### Response

```json
{
  "ok": true
}
```

### 状态枚举

```text
processing
running_tool
completed
failed
```

该 API 可选。若第一阶段前端不展示处理状态，可以延后实现。

## API 3：查询会话上下文

```text
GET /internal/agent/conversations/{conversation_id}/context?before_seq=2033&limit=50
```

### Response

```json
{
  "conversation_id": "group_789",
  "conversation_type": "group",
  "members": [
    {
      "member_type": "user",
      "member_id": "user_123",
      "nickname": "Junhui"
    },
    {
      "member_type": "agent",
      "member_id": "agent_456",
      "nickname": "Assistant"
    }
  ],
  "messages": [
    {
      "message_id": "msg_456",
      "seq": 2033,
      "sender_type": "user",
      "sender_id": "user_123",
      "content_type": "text",
      "content": {
        "type": "text",
        "text": "@agent_456 总结一下今天的讨论"
      },
      "created_at": "2026-04-28T00:00:00Z"
    }
  ]
}
```

### 规则

- 只返回 Agent 有权限访问的会话上下文。
- 默认按 `seq` 倒序查询，再由调用方按需要排序。
- 第一阶段 `limit` 最大 100。

## API 4：查询 Agent 会话绑定

```text
GET /internal/agent/conversations/{conversation_id}/agents
```

### Response

```json
{
  "conversation_id": "group_789",
  "agents": [
    {
      "agent_id": "agent_456",
      "nickname": "Assistant",
      "status": "enabled",
      "respond_policy": "mention_only"
    }
  ]
}
```

## 消息类型约定

第一阶段最小消息类型：

| 类型 | 说明 |
| --- | --- |
| `text` | 普通文本消息 |
| `agent_text` | Agent 文本消息，可映射为 `text + sender_type=agent` |
| `tool_result` | Agent 工具调用结果 |
| `system` | 系统消息 |

### Tool Result Content

```json
{
  "type": "tool_result",
  "tool_name": "web_search",
  "status": "success",
  "summary": "搜索完成，找到 3 条相关结果。",
  "data_ref": "tool_result_123"
}
```

## Agent 触发策略

第一阶段支持三种触发：

| trigger | 场景 |
| --- | --- |
| `direct_chat` | 用户与 Agent 单聊 |
| `mention` | 群聊中 @Agent |
| `auto` | 群配置允许 Agent 自动响应，第一阶段可暂缓 |

默认策略：

- 单聊 Agent 必须响应。
- 群聊只有 @Agent 时响应。
- 多 Agent 同时被 @ 时，分别触发对应 Agent，但需要共享同一个 `trace_id`。

## 幂等规则

- IM -> Agent：Agent 系统以 `event_id` 幂等。
- Agent -> IM message writeback：IM 后端以 `request_id` 幂等。
- 同一个 `event_id` 重复投递时，Agent 不应重复执行工具调用；可以返回上一次处理结果或直接返回成功。
- 同一个 `request_id` 重复写回时，IM 返回同一个 `message_id`。

## 错误与重试

### IM -> Agent

- HTTP 2xx 且 `action_code=0`：成功。
- HTTP 2xx 但 `action_code!=0`：业务失败，根据 `err_code` 决定是否重试。
- HTTP 408/429/5xx：可重试。
- HTTP 400/401/403：不可重试，记录失败。

### Agent -> IM

- HTTP 2xx：成功。
- HTTP 409：幂等冲突或状态冲突，Agent 需要查询结果。
- HTTP 401/403：鉴权或权限错误，不重试。
- HTTP 429/5xx：可重试。

## 安全要求

- IM -> Agent webhook 必须校验 `X-IM-Signature`。
- Agent -> IM 内部 API 必须使用服务间 token 或 mTLS。
- 所有请求必须携带 `trace_id`。
- 日志不得记录明文 token、签名密钥或敏感消息全文；必要时记录摘要或 message_id。
- 工具调用结果写回前必须经过 Agent 系统的安全过滤。

## 第一阶段开发分工

### IM 后端可先实现

- webhook 配置结构。
- `callbackAfterSendSingleMsgCommand`。
- `callbackAfterSendGroupMsgCommand`。
- `/internal/agent/messages`。
- `/internal/agent/conversations/{conversation_id}/context`。
- `event_id`、`request_id` 幂等。

### Agent 系统可先实现

- webhook 接收服务。
- 签名校验。
- `message.created` 事件解析。
- 单聊 Agent 响应。
- 群聊 @Agent 响应。
- Agent 消息写回 IM。

### 前端可先实现

- 展示 `sender_type=agent` 的消息。
- 展示 `tool_result` 消息。
- 群聊 @Agent 输入体验。
- Agent `processing` 状态展示，可选。

## 待确认问题

- IM -> Agent 是否最终使用 HTTP webhook、Kafka topic，或两者结合。
- Agent -> IM 是否长期使用 REST，还是迁移到 gRPC。
- OpenIM `contentType` 数字枚举是否保留，还是在本项目统一为字符串枚举。
- Agent 状态消息是否需要持久化。
- 工具调用结果是否完整入 IM 消息表，还是只保存摘要和 `data_ref`。
