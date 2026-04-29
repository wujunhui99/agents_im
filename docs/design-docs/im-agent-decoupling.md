# IM 与 Agent 解耦设计

状态：Draft

## 背景

IM 核心链路需要保持低延迟和高可靠，而 Agent 推理和工具调用可能耗时较长、失败模式复杂。因此，Agent 不应阻塞 IM 核心消息链路。

## 目标

- IM Core 与 Agent Service 通过事件和 Webhook 解耦。
- Agent 响应异步写回 IM。
- 所有事件具备幂等性、可追踪性和可重试能力。

## 初步方案

1. IM Core 在消息写入后生成消息事件。
2. 事件写入 Kafka 或事件表。
3. Webhook Dispatcher 投递事件到 Agent Service。
4. Agent Service 处理事件并产生回复消息。
5. 回复消息通过 IM Core 标准写入链路进入会话。

## 关键约束

- 所有事件必须携带 `trace_id`。
- Webhook 投递必须支持重试和幂等键。
- Agent 超时不影响用户消息落库。
