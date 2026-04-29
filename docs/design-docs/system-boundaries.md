# 系统职责与边界设计

状态：Accepted

本文档定义 IM 后端、Agent 系统和前端系统的职责边界。边界明确后，IM 后端和 Agent 系统可以并行开发；前端也可以基于接口契约和 Mock 服务并行推进。

## 总体结论

系统拆分为三个主要方向：

1. **IM 后端系统**：负责用户、会话、消息、长连接、投递可靠性和基础 IM 能力。
2. **Agent 系统**：负责 Agent 生命周期、Agent 推理、工具调用、Agent 与 IM 的异步集成。
3. **前端系统**：负责用户交互、实时消息展示、连接状态展示、Agent 交互入口和工具结果呈现。

核心解耦原则：

```text
Frontend -> IM Backend -> Webhook/Event -> Agent System -> IM Backend -> Frontend
```

IM 后端不直接承载 Agent 推理；Agent 系统不直接管理 WebSocket 长连接；前端不直接调用 Agent 内部运行接口。

## IM 后端系统职责

IM 后端是系统的实时通信底座，目标是提供稳定、可靠、可扩展的聊天能力。

### 负责内容

- 用户与身份相关的基础模型。
- 会话模型，包括单聊、群聊、会话成员、成员角色等。
- 消息模型，包括文本消息、系统消息、Agent 消息、工具结果消息等。
- 消息写入、查询、分页、撤回、状态更新等基础能力。
- WebSocket 长连接管理。
- 心跳、ACK、重连、离线消息补偿等可靠性机制。
- 在线状态和连接状态维护。
- Kafka 消息事件生产与消费。
- PostgreSQL 与 Redis 的核心数据存储。
- Webhook/Event 分发，将 IM 事件异步通知 Agent 系统。
- 接收 Agent 系统写回的消息，并走统一 IM 消息链路投递。
- IM 侧鉴权、权限校验和幂等控制。
- IM 核心链路指标、日志和 tracing。

### 不负责内容

- 不负责 Agent 的具体推理逻辑。
- 不负责 LLM 调用编排。
- 不负责 Agent 工具执行细节。
- 不负责 Agent 内部 memory、planner、evaluator 等运行时逻辑。
- 不直接暴露 Agent 内部管理接口给前端。

### 对外契约

IM 后端需要向前端提供：

- 登录/鉴权接口。
- 会话列表接口。
- 消息历史接口。
- WebSocket 实时消息协议。
- 消息发送接口或 WebSocket send 协议。
- ACK/重试/状态同步协议。
- Agent 成员展示所需的会话成员信息。

IM 后端需要向 Agent 系统提供：

- IM 事件 Webhook 或事件订阅。
- Agent 写回消息接口。
- 会话上下文查询接口。
- Agent 可用的 IM 工具接口，例如发送消息、读取上下文、查询成员信息。

## Agent 系统职责

Agent 系统是智能体运行和工具调用中心，目标是让 Agent 能够稳定、安全、可追踪地参与 IM 会话。

### 负责内容

- Agent 创建、销毁、启用、禁用、配置更新和持久化。
- Agent profile、system prompt、能力配置、工具权限配置。
- Agent 与会话的绑定关系。
- Agent 接收 IM 事件并判断是否需要响应。
- Agent 单聊响应逻辑。
- Agent 群聊响应逻辑，包括 @Agent、多 Agent 协同和多用户上下文。
- Planner / Generator / Evaluator 执行链路。
- LLM 调用编排。
- 工具调用体系，包括代码执行、网络搜索、IM 工具调用等。
- Agent memory 和上下文管理。
- 工具调用审计、权限控制和结果结构化。
- 将 Agent 响应写回 IM 后端。
- Agent 侧指标、日志和 tracing。

### 不负责内容

- 不负责 WebSocket 连接管理。
- 不负责普通用户消息的最终投递。
- 不负责 IM 基础数据模型的主存储。
- 不绕过 IM 后端直接向前端推送消息。
- 不直接修改 IM 核心数据，所有写入应通过 IM 后端提供的接口或事件契约完成。

### 对外契约

Agent 系统需要接收 IM 后端提供的：

- 新消息事件。
- 会话成员变更事件。
- Agent 被邀请或移除事件。
- 用户 @Agent 事件或可推导的消息事件。
- 会话上下文查询能力。

Agent 系统需要向 IM 后端提供：

- Agent 消息写回。
- Agent typing / processing 状态，可选。
- 工具调用结果消息。
- Agent 错误或降级响应。

## 前端系统职责

前端系统是用户交互层，目标是提供清晰、可靠、可理解的实时聊天和 Agent 协作体验。

### 负责内容

- 登录态和用户身份展示。
- 会话列表、会话详情、群成员展示。
- WebSocket 连接建立、断线重连和连接状态展示。
- 消息发送、消息列表渲染、消息状态展示。
- ACK、发送中、发送失败、重试等状态展示。
- Agent 成员展示。
- @Agent 交互入口。
- Agent 正在处理、工具调用中、工具结果等 UI 表达。
- 多 Agent 场景下区分不同 Agent 身份和职责。
- 前端可观测性，例如关键错误上报和用户行为事件。

### 不负责内容

- 不直接调用 LLM。
- 不直接执行 Agent 工具。
- 不保存核心消息数据的最终状态。
- 不自行判断 Agent 是否应该响应，最多提供用户显式触发信息，例如 @Agent。
- 不绕过 IM 后端直接调用 Agent 内部运行接口。

### 对外契约

前端主要依赖 IM 后端：

- REST/gRPC Gateway API。
- WebSocket 协议。
- 消息类型 schema。
- 会话与成员 schema。
- Agent 展示字段。

前端与 Agent 系统默认不直接通信。如确需前端直接访问 Agent 管理接口，必须先在设计文档中明确安全边界和鉴权方案。

## 三方边界矩阵

| 能力 | IM 后端 | Agent 系统 | 前端 |
| --- | --- | --- | --- |
| WebSocket 长连接 | Owner | 不参与 | Client |
| 消息持久化 | Owner | 通过 IM 写入 | 展示 |
| 消息投递 | Owner | 不参与 | 接收与 ACK |
| Agent 生命周期 | 提供会话绑定支持 | Owner | 管理入口，可选 |
| Agent 推理 | 不参与 | Owner | 不参与 |
| Agent 工具调用 | 提供 IM 工具接口 | Owner | 展示结果 |
| 会话上下文 | Owner | 查询/使用 | 展示 |
| 群聊成员管理 | Owner | 订阅事件 | 展示/操作 |
| 鉴权 | Owner | 校验调用来源 | 保存登录态 |
| tracing | 传递 `trace_id` | 传递 `trace_id` | 展示/上报 request id，可选 |

## 可并行开发的前提

IM 后端和 Agent 系统可以并行开发，但必须先稳定以下契约：

1. **事件契约**：IM 如何通知 Agent。
2. **消息写回契约**：Agent 如何把响应写回 IM。
3. **会话上下文契约**：Agent 如何读取必要上下文。
4. **Agent 成员模型**：IM 如何表示 Agent 作为会话成员。
5. **消息类型 schema**：如何区分用户消息、Agent 消息、工具结果消息、系统消息。
6. **鉴权与签名机制**：Webhook 和内部 API 如何认证。
7. **trace_id 传递规则**：跨 IM、Agent、工具调用链路统一追踪。

前端可以在消息协议和 UI schema 初步确定后并行开发，并通过 Mock IM API / Mock WebSocket / Mock Agent 消息进行联调。

## 建议的并行开发切分

### IM Agent A：IM 后端主线

- 用户、会话、消息基础模型。
- WebSocket 连接和消息收发。
- ACK、心跳、重连和离线补偿。
- Kafka 事件流。
- Agent 成员作为特殊会话成员。
- Webhook/Event Dispatcher。

### Agent Agent B：Agent 系统主线

- Agent 生命周期管理。
- IM 事件接收接口。
- Agent 响应生成流程。
- 工具调用抽象。
- Agent 消息写回 IM。
- Agent 单聊和群聊基础逻辑。

### Frontend Agent C：前端主线

- 登录和会话列表。
- WebSocket 消息收发。
- 消息状态展示。
- Agent 消息和工具结果展示。
- @Agent 交互。
- 多 Agent 群聊 UI。

## 第一阶段必须冻结的最小契约

为支持 IM 和 Agent 并行开发，第一阶段建议先冻结以下最小契约。

### MessageCreated Event

```json
{
  "event_id": "evt_123",
  "event_type": "message.created",
  "trace_id": "trace_123",
  "conversation_id": "conv_123",
  "message_id": "msg_123",
  "sender": {
    "type": "user",
    "id": "user_123"
  },
  "content": {
    "type": "text",
    "text": "@agent 帮我总结一下"
  },
  "mentions": [
    {
      "type": "agent",
      "id": "agent_123"
    }
  ],
  "created_at": "2026-04-28T00:00:00Z"
}
```

### Agent Message Writeback

```json
{
  "request_id": "req_123",
  "trace_id": "trace_123",
  "conversation_id": "conv_123",
  "agent_id": "agent_123",
  "reply_to_message_id": "msg_123",
  "content": {
    "type": "text",
    "text": "这是总结结果。"
  },
  "metadata": {
    "tool_calls": []
  }
}
```

### Tool Result Message

```json
{
  "type": "tool_result",
  "tool_name": "web_search",
  "status": "success",
  "summary": "搜索完成",
  "data_ref": "tool_result_123"
}
```

## 待进一步设计

- IM 与 Agent 之间使用 HTTP Webhook、Kafka Consumer Group，还是两者结合。
- Agent 写回 IM 使用内部 REST API 还是 gRPC。
- Agent memory 的存储边界。
- 多 Agent 同时响应时的排序、去重和降噪策略。
- 工具调用结果在 IM 消息中的结构化展示方式。
- 前端是否需要 Agent 管理后台。
