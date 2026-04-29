# RELIABILITY.md

本文档记录可靠性目标和工程约束。

## 可靠性目标

- 消息写入成功后，系统应尽最大可能完成投递或提供可追踪的失败状态。
- WebSocket 断连后支持重连和消息补偿。
- Agent 处理失败不应影响 IM 核心消息链路。
- Kafka、Redis、PostgreSQL 的故障应有降级或恢复策略。

## 关键机制

- WebSocket 心跳
- 消息 ACK
- 幂等写入
- 事件重试
- 死信队列或失败事件表
- `trace_id` 全链路追踪

## 待补充

- SLO / SLA
- 告警规则
- 压测目标
- 故障演练方案
